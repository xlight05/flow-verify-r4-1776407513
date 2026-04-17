package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/asdlc/todo-api/internal/models"
)

var ErrNotFound = errors.New("todo not found")

// Store is an in-memory todo store with optional JSON-file persistence.
type Store struct {
	mu   sync.RWMutex
	path string
	data map[string]models.Todo
}

// New creates a Store. If path is non-empty, it attempts to load/persist there;
// if the directory or file is not usable, it degrades gracefully to in-memory.
func New(path string) (*Store, error) {
	s := &Store{path: path, data: make(map[string]models.Todo)}
	if path == "" {
		return s, nil
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf(`{"level":"warn","msg":"data dir not writable; running in-memory","path":%q,"err":%q}`, dir, err.Error())
			s.path = ""
			return s, nil
		}
	}
	if err := s.load(); err != nil {
		log.Printf(`{"level":"warn","msg":"data file load failed; running in-memory","path":%q,"err":%q}`, path, err.Error())
		s.path = ""
	}
	return s, nil
}

func (s *Store) load() error {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open data file: %w", err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	if len(b) == 0 {
		return nil
	}
	var list []models.Todo
	if err := json.Unmarshal(b, &list); err != nil {
		return fmt.Errorf("decode data file: %w", err)
	}
	for _, t := range list {
		s.data[t.ID] = t
	}
	return nil
}

func (s *Store) persistLocked() error {
	if s.path == "" {
		return nil
	}
	dir := filepath.Dir(s.path)

	list := make([]models.Todo, 0, len(s.data))
	for _, t := range s.data {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })

	tmp, err := os.CreateTemp(dir, ".todos-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(list); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("encode todos: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func (s *Store) List() []models.Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Todo, 0, len(s.data))
	for _, t := range s.data {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Get(id string) (models.Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.data[id]
	if !ok {
		return models.Todo{}, ErrNotFound
	}
	return t, nil
}

func (s *Store) Add(t models.Todo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[t.ID] = t
	if err := s.persistLocked(); err != nil {
		delete(s.data, t.ID)
		return err
	}
	return nil
}

// Complete marks the todo completed. Idempotent: if already completed, the
// stored record is returned unchanged and changed=false.
func (s *Store) Complete(id string) (todo models.Todo, changed bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok {
		return models.Todo{}, false, ErrNotFound
	}
	if t.Completed {
		return t, false, nil
	}
	prev := t
	now := time.Now().UTC()
	t.Completed = true
	t.CompletedAt = &now
	s.data[id] = t
	if perr := s.persistLocked(); perr != nil {
		s.data[id] = prev
		return models.Todo{}, false, perr
	}
	return t, true, nil
}

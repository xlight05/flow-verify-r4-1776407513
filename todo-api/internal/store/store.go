package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/asdlc/todo-api/internal/models"
)

var ErrNotFound = errors.New("todo not found")

type Store struct {
	mu   sync.RWMutex
	path string
	data []models.Todo
}

func New(path string) (*Store, error) {
	s := &Store{path: path}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = []models.Todo{}
			return nil
		}
		return fmt.Errorf("open data file: %w", err)
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	if len(bytes) == 0 {
		s.data = []models.Todo{}
		return nil
	}
	var todos []models.Todo
	if err := json.Unmarshal(bytes, &todos); err != nil {
		return fmt.Errorf("decode data file: %w", err)
	}
	s.data = todos
	return nil
}

func (s *Store) persistLocked() error {
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".todos-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.data); err != nil {
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
	out := make([]models.Todo, len(s.data))
	copy(out, s.data)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *Store) Add(todo models.Todo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = append(s.data, todo)
	if err := s.persistLocked(); err != nil {
		s.data = s.data[:len(s.data)-1]
		return err
	}
	return nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, t := range s.data {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrNotFound
	}
	removed := s.data[idx]
	s.data = append(s.data[:idx], s.data[idx+1:]...)
	if err := s.persistLocked(); err != nil {
		restored := append([]models.Todo{}, s.data[:idx]...)
		restored = append(restored, removed)
		restored = append(restored, s.data[idx:]...)
		s.data = restored
		return err
	}
	return nil
}

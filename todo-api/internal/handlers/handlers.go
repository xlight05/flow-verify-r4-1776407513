package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/asdlc/todo-api/internal/auth"
	"github.com/asdlc/todo-api/internal/metrics"
	"github.com/asdlc/todo-api/internal/middleware"
	"github.com/asdlc/todo-api/internal/models"
	"github.com/asdlc/todo-api/internal/store"
	"github.com/asdlc/todo-api/internal/uuid"
)

type Handler struct {
	Store *store.Store
}

func New(s *store.Store) *Handler {
	return &Handler{Store: s}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/todos", h.todosCollection)
	mux.HandleFunc("/todos/", h.todosItem)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, models.APIError{Code: code, Message: msg})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// requireUser parses and validates the bearer token. On failure it writes the
// 401 response and returns false.
func (h *Handler) requireUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	token := auth.BearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
		return "", false
	}
	middleware.Fields(r).UserID = token
	return token, true
}

func (h *Handler) todosCollection(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		all := h.Store.List()
		out := make([]models.Todo, 0, len(all))
		for _, t := range all {
			if t.OwnerID == user {
				out = append(out, t)
			}
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		h.createTodo(w, r, user)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h *Handler) createTodo(w http.ResponseWriter, r *http.Request, user string) {
	var payload struct {
		Title string `json:"title"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "title is required")
		return
	}
	id, err := uuid.New()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to generate id")
		return
	}
	todo := models.Todo{
		ID:      id,
		Title:   title,
		OwnerID: user,
	}
	if err := h.Store.Add(todo); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to persist todo")
		return
	}
	middleware.Fields(r).TodoID = id
	writeJSON(w, http.StatusCreated, todo)
}

func (h *Handler) todosItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/todos/")
	if rest == "" {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	middleware.Fields(r).TodoID = id

	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}

	switch action {
	case "":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
		h.getTodo(w, r, user, id)
	case "complete":
		if r.Method != http.MethodPatch {
			methodNotAllowed(w, "PATCH")
			return
		}
		h.completeTodo(w, r, user, id)
	default:
		writeError(w, http.StatusNotFound, "not_found", "not found")
	}
}

func (h *Handler) getTodo(w http.ResponseWriter, r *http.Request, user, id string) {
	if !uuid.Valid(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID")
		return
	}
	t, err := h.Store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "todo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "unexpected error")
		return
	}
	if t.OwnerID != user {
		writeError(w, http.StatusForbidden, "forbidden", "not the owner of this todo")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) completeTodo(w http.ResponseWriter, r *http.Request, user, id string) {
	start := time.Now()
	var status int
	defer func() {
		metrics.CompleteLatency.Observe(time.Since(start).Seconds())
		if status != 0 {
			metrics.CompleteRequests.WithLabelValues(strconv.Itoa(status)).Inc()
		}
	}()

	if !uuid.Valid(id) {
		status = http.StatusBadRequest
		writeError(w, status, "invalid_id", "id must be a UUID")
		return
	}

	if r.Body != nil {
		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var empty struct{}
		if err := dec.Decode(&empty); err != nil && !errors.Is(err, io.EOF) {
			status = http.StatusBadRequest
			writeError(w, status, "invalid_body", "body must be empty or {}")
			return
		}
	}

	existing, err := h.Store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
			writeError(w, status, "not_found", "todo not found")
			return
		}
		status = http.StatusInternalServerError
		writeError(w, status, "internal_error", "unexpected error")
		return
	}
	if existing.OwnerID != user {
		status = http.StatusForbidden
		writeError(w, status, "forbidden", "not the owner of this todo")
		return
	}

	updated, _, err := h.Store.Complete(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
			writeError(w, status, "not_found", "todo not found")
			return
		}
		status = http.StatusInternalServerError
		writeError(w, status, "internal_error", "failed to update todo")
		return
	}
	status = http.StatusOK
	writeJSON(w, status, updated)
}

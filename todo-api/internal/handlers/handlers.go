package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

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
	mux.HandleFunc("/", h.notFound)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, models.ErrorResponse{Message: msg})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) todosCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		todos := h.Store.List()
		writeJSON(w, http.StatusOK, todos)
	case http.MethodPost:
		h.createTodo(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h *Handler) createTodo(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Title string `json:"title"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	id, err := uuid.New()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	todo := models.Todo{
		ID:        id,
		Title:     title,
		Completed: false,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.Store.Add(todo); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, todo)
}

func (h *Handler) todosItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/todos/")
	if id == "" || strings.Contains(id, "/") {
		h.notFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := h.Store.Delete(id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "todo not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, "DELETE")
	}
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}

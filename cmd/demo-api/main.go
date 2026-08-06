package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ayasqualli/go-sqlite-crud-kit/taskstore"
)

type server struct {
	store taskstore.Repository
}

type createTaskInput struct {
	Title string `json:"title"`
}

type updateTaskInput struct {
	Title *string `json:"title,omitempty"`
	Done  *bool   `json:"done,omitempty"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	path := os.Getenv("DB_PATH")
	if path == "" {
		path = "tasks.db"
	}
	store, err := taskstore.OpenSQLite(ctx, taskstore.Config{Path: path})
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	s := &server{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /tasks", s.listTasks)
	mux.HandleFunc("GET /tasks/{id}", s.getTask)
	mux.HandleFunc("POST /tasks", s.createTask)
	mux.HandleFunc("PUT /tasks/{id}", s.updateTask)
	mux.HandleFunc("DELETE /tasks/{id}", s.deleteTask)
	mux.HandleFunc("GET /stats", s.stats)
	mux.HandleFunc("POST /reset", s.reset)

	addr := ":3000"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	log.Printf("demo API listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) listTasks(w http.ResponseWriter, r *http.Request) {
	opts := taskstore.ListOptions{Search: r.URL.Query().Get("search")}
	if raw := r.URL.Query().Get("done"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "done must be true or false")
			return
		}
		opts.Done = &value
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		opts.Limit = value
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "offset must be an integer")
			return
		}
		opts.Offset = value
	}

	tasks, total, err := s.store.List(r.Context(), opts)
	if errors.Is(err, taskstore.ErrInvalidList) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, tasks)
}

func (s *server) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	task, err := s.store.Get(r.Context(), id)
	if errors.Is(err, taskstore.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	var input createTaskInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	task, err := s.store.Create(r.Context(), input.Title)
	if errors.Is(err, taskstore.ErrInvalidTitle) {
		writeError(w, http.StatusBadRequest, "title is required and cannot be empty")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/tasks/%d", task.ID))
	writeJSON(w, http.StatusCreated, task)
}

func (s *server) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var input updateTaskInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	task, err := s.store.Update(r.Context(), id, taskstore.Update{Title: input.Title, Done: input.Done})
	switch {
	case errors.Is(err, taskstore.ErrNotFound):
		writeError(w, http.StatusNotFound, "Task not found")
	case errors.Is(err, taskstore.ErrInvalidTitle), errors.Is(err, taskstore.ErrNoFields):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "database error")
	default:
		writeJSON(w, http.StatusOK, task)
	}
}

func (s *server) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	err := s.store.Delete(r.Context(), id)
	if errors.Is(err, taskstore.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *server) reset(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.Reset(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "Task id must be a positive integer")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

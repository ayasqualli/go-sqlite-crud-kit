// Package taskstore is a database-backed example repository built on top of
// sqlitekit. It contains no HTTP code, so it can be reused by net/http, Gin,
// Fiber, gRPC, CLI, or background-worker projects.
package taskstore

import (
	"context"
	"errors"
)

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalidTitle = errors.New("task title must not be empty")
	ErrNoFields     = errors.New("task update must include title or done")
	ErrInvalidList  = errors.New("invalid list options")
)

const (
	DefaultLimit = 50
	MaxLimit     = 100
)

type Task struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type SeedTask struct {
	Title string
	Done  bool
}

type Update struct {
	Title *string
	Done  *bool
}

type ListOptions struct {
	Done   *bool
	Search string
	Limit  int
	Offset int
}

type Stats struct {
	Total int `json:"total"`
	Done  int `json:"done"`
	Open  int `json:"open"`
}

// Repository is the boundary the API depends on. A future Postgres or in-memory
// implementation can satisfy the same interface without changing HTTP routes.
type Repository interface {
	List(context.Context, ListOptions) ([]Task, int, error)
	Get(context.Context, int64) (Task, error)
	Create(context.Context, string) (Task, error)
	Update(context.Context, int64, Update) (Task, error)
	Delete(context.Context, int64) error
	Stats(context.Context) (Stats, error)
	Reset(context.Context, []SeedTask) ([]Task, error)
	Close() error
}

var DefaultSeed = []SeedTask{
	{Title: "Learn HTTP basics", Done: true},
	{Title: "Build a CRUD API", Done: false},
	{Title: "Connect CRUD to SQLite", Done: false},
}

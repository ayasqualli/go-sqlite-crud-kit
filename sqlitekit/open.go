// The package sqlitekit provides the resuable SQLite connection setup used by repos
// in this project. It intentionally knows nothing about tasks or HTTP, so it can be
// imported by other applications
package sqlitekit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Config controls how an SQLite database is opened
// Path is the database filename or SQLite DSN. An empty value uses app.db
// MaxOpenConns defaults to 1 (safe and simple for small apps)
type Config struct {
	Path         string
	MaxOpenConns int
}

// Open creates or opens an SQLite database and applies safe defaults.
// The caller owns the returned *sql.DB and must close it
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		path = "app.db"
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlitedatabase: %w", err)
	}

	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 1
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("configure sqlite database: %w", err)
		}
	}
	return db, nil
}

// RunInTransaction executes fn atomically. A panic or returned errors rolls the transaction back, commited if successful
func RunInTransaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) (err error) {
	if db == nil {
		return errors.New("sqlitekit: nil database")
	}
	if fn == nil {
		return errors.New("sqlitekit: nil transaction fucntion ")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

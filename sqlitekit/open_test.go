package sqlitekit

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseAndTransactionCommits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "app.db")

	db, err := Open(ctx, Config{Path: path})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := RunInTransaction(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO items(name) VALUES (?)`, "example")
		return err
	}); err != nil {
		t.Fatalf("RunInTransaction() error = %v", err)
	}

	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM items WHERE id = 1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "example" {
		t.Fatalf("name = %q, want example", name)
	}
}

func TestRunInTransactionRollback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rollback.db")

	db, err := Open(ctx, Config{Path: path})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `CREATE TABLE values_table (value TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	want := errors.New("stop")
	err = RunInTransaction(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO values_table(value) VALUES (?)`, "temporary"); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("RunInTransaction() error = %v, want %v", err, want)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM values_table`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 after rollback", count)
	}
}

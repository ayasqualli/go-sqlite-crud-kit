package taskstore

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) (*SQLiteStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.db")
	store, err := OpenSQLite(context.Background(), Config{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	return store, path
}

func TestSeedRunsOnlyWhenDatabaseIsEmptyAndPersists(t *testing.T) {
	ctx := context.Background()
	store, path := openTestStore(t)

	tasks, total, err := store.List(ctx, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(tasks) != 3 {
		t.Fatalf("initial tasks = %d/%d, want 3/3", len(tasks), total)
	}

	created, err := store.Create(ctx, "Persistent task")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	tasks, total, err = reopened.List(ctx, ListOptions{Limit: MaxLimit})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(tasks) != 4 {
		t.Fatalf("reopened tasks = %d/%d, want 4/4", len(tasks), total)
	}
	if tasks[3].ID != created.ID || tasks[3].Title != "Persistent task" {
		t.Fatalf("persistent task = %+v, created = %+v", tasks[3], created)
	}
}

func TestCRUDFilterSearchStatsAndReset(t *testing.T) {
	ctx := context.Background()
	store, _ := openTestStore(t)
	defer store.Close()

	created, err := store.Create(ctx, "Buy milk")
	if err != nil {
		t.Fatal(err)
	}
	if created.Done {
		t.Fatal("new task should be open")
	}

	title := "Buy oat milk"
	done := true
	updated, err := store.Update(ctx, created.ID, Update{Title: &title, Done: &done})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != title || !updated.Done {
		t.Fatalf("updated task = %+v", updated)
	}

	filtered, total, err := store.List(ctx, ListOptions{Done: &done, Search: "oat", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(filtered) != 1 || filtered[0].ID != created.ID {
		t.Fatalf("filtered tasks = %+v, total = %d", filtered, total)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 4 || stats.Done != 2 || stats.Open != 2 {
		t.Fatalf("stats = %+v, want total=4 done=2 open=2", stats)
	}

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}

	reset, err := store.Reset(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(reset) != 3 || reset[0].ID != 1 {
		t.Fatalf("reset tasks = %+v", reset)
	}
}

func TestValidationAndNotFound(t *testing.T) {
	ctx := context.Background()
	store, _ := openTestStore(t)
	defer store.Close()

	if _, err := store.Create(ctx, "   "); !errors.Is(err, ErrInvalidTitle) {
		t.Fatalf("Create() error = %v, want ErrInvalidTitle", err)
	}
	if _, err := store.Update(ctx, 1, Update{}); !errors.Is(err, ErrNoFields) {
		t.Fatalf("Update() error = %v, want ErrNoFields", err)
	}
	if _, err := store.Get(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
	if err := store.Delete(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ErrNotFound", err)
	}
	if _, _, err := store.List(ctx, ListOptions{Limit: MaxLimit + 1}); !errors.Is(err, ErrInvalidList) {
		t.Fatalf("List() error = %v, want ErrInvalidList", err)
	}
}

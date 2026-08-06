# Go SQLite CRUD Kit

A standalone persistence component for Go projects. It separates database work from HTTP routes so the same storage layer can be reused by a `net/http` API now and added to other projects later.

```text
HTTP handlers / CLI / worker
            |
            v
  taskstore.Repository
            |
            v
    SQLiteStore + SQL
            |
            v
         tasks.db
```

The demo keeps the same task CRUD behavior while moving storage from a Go slice to SQLite: automatic database creation, automatic schema creation, one-time seeding, parameterized queries, persistence across restarts, and correct not-found signaling.

## What is reusable

- `sqlitekit`: generic SQLite connection and transaction helpers; no task-specific code.
- `taskstore`: a complete repository example for tasks.
- `taskstore.Repository`: the interface your API depends on, allowing a future SQLite, Postgres, test, or in-memory implementation.
- `cmd/demo-api`: a runnable `net/http` example, not required when importing the packages elsewhere.

## Requirements

- Go 1.22 or newer
- No separate SQLite server
- The project uses the CGo-free `modernc.org/sqlite` driver, so it works without installing a C compiler.

## Run the standalone demo

```bash
go mod tidy
go run ./cmd/demo-api
```

Then:

```bash
curl -i http://localhost:3000/tasks
```

The first run creates `tasks.db`, creates the `tasks` table, and inserts exactly three seed rows. Restarting the process does not duplicate the seed and does not remove tasks you created.

Run the full demonstration in another terminal:

```bash
./scripts/demo.sh
```

## Use it from another local project

Place the repositories next to each other:

```text
workspace/
├── go-sqlite-crud-kit/
└── my-api/
```

Inside `my-api`:

```bash
go mod edit -replace github.com/your-username/go-sqlite-crud-kit=../go-sqlite-crud-kit
go get github.com/your-username/go-sqlite-crud-kit/taskstore
go mod tidy
```

Startup code:

```go
store, err := taskstore.OpenSQLite(context.Background(), taskstore.Config{
    Path: "tasks.db",
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()

app := NewApp(store) // NewApp should accept taskstore.Repository
```

See [`docs/INTEGRATION.md`](docs/INTEGRATION.md) for the exact interface injection and HTTP error mapping pattern.

## Database schema

```sql
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    done INTEGER NOT NULL DEFAULT 0 CHECK (done IN (0, 1))
);
```

All user values are passed through `?` placeholders rather than concatenated into SQL strings.

## Package API

```go
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
```

The API layer maps `taskstore.ErrNotFound` to HTTP `404`, validation errors to `400`, and successful deletes to `204`. Database errors remain internal and should map to `500`.

## Verify persistence

```bash
# Start the API and create a row
go run ./cmd/demo-api
curl -X POST http://localhost:3000/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"This survives"}'

# Stop and restart the process, then list again
curl http://localhost:3000/tasks
```

Open `tasks.db` in DB Browser for SQLite and run:

```sql
SELECT * FROM tasks;
SELECT * FROM tasks WHERE done = 1;
SELECT COUNT(*) FROM tasks;
```

## Tests

```bash
make check
```

The test suite checks automatic setup, one-time seeding, persistence after reopening the file, full CRUD, search/filtering, SQL statistics, reset behavior, validation, transactions, and not-found errors.

## Project structure

```text
.
├── sqlitekit/             # Generic SQLite setup and transaction helper
├── taskstore/             # Task repository and SQL implementation
├── cmd/demo-api/          # Runnable net/http integration example
├── scripts/demo.sh        # CRUD verification flow
├── docs/INTEGRATION.md    # Add the module to another Go project
├── docs/ASSIGNMENT_NOTES.md
├── Makefile
└── go.mod
```

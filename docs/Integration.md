# Integration guide

The reusable boundary is `taskstore.Repository`. Your HTTP layer depends on the interface, not on SQLite directly.

## 1. Open the store during application startup

```go
store, err := taskstore.OpenSQLite(ctx, taskstore.Config{
    Path: "tasks.db",
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

## 2. Inject the interface into your application

```go
type App struct {
    store taskstore.Repository
}

func NewApp(store taskstore.Repository) *App {
    return &App{store: store}
}
```

This is the only architectural change your route layer needs. The endpoints stay the same.

## 3. Map repository errors to HTTP status codes

```go
task, err := app.store.Get(r.Context(), id)
switch {
case errors.Is(err, taskstore.ErrNotFound):
    writeError(w, http.StatusNotFound, "Task not found")
case err != nil:
    writeError(w, http.StatusInternalServerError, "database error")
default:
    writeJSON(w, http.StatusOK, task)
}
```

Keep request validation and HTTP response formatting in the API project. Keep SQL, schema creation, seeding, and persistence in this module.

## 4. Adapting the pattern to another entity

For projects with users, reports, products, or incidents:

1. Keep `sqlitekit` unchanged.
2. Copy `taskstore` to a new package such as `userstore` or `reportstore`.
3. Replace the schema and SQL queries.
4. Keep a small repository interface so the API does not depend on a particular database.

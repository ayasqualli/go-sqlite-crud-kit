package taskstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ayasqualli/go-sqlite-crud-kit/sqlitekit"
)

type Config struct {
	Path string
	// Seed nil uses DefaultSeed. An explicit empty slice disables seeding.
	Seed []SeedTask
}

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(ctx context.Context, cfg Config) (*SQLiteStore, error) {
	if strings.TrimSpace(cfg.Path) == "" {
		cfg.Path = "tasks.db"
	}
	db, err := sqlitekit.Open(ctx, sqlitekit.Config{Path: cfg.Path})
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	if err := store.initialize(ctx, cfg.Seed); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) initialize(ctx context.Context, seed []SeedTask) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    done INTEGER NOT NULL DEFAULT 0 CHECK (done IN (0, 1))
)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_done ON tasks(done)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_title ON tasks(title COLLATE NOCASE)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize task schema: %w", err)
		}
	}

	if seed == nil {
		seed = DefaultSeed
	}
	return s.seedIfEmpty(ctx, seed)
}

func (s *SQLiteStore) seedIfEmpty(ctx context.Context, seed []SeedTask) error {
	return sqlitekit.RunInTransaction(ctx, s.db, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&count); err != nil {
			return fmt.Errorf("count tasks before seed: %w", err)
		}
		if count != 0 {
			return nil
		}
		for _, item := range seed {
			title := strings.TrimSpace(item.Title)
			if title == "" {
				return ErrInvalidTitle
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO tasks (title, done) VALUES (?, ?)`,
				title, boolInt(item.Done),
			); err != nil {
				return fmt.Errorf("seed task: %w", err)
			}
		}
		return nil
	})
}

func (s *SQLiteStore) List(ctx context.Context, opts ListOptions) ([]Task, int, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = DefaultLimit
	}
	if limit < 1 || limit > MaxLimit || opts.Offset < 0 {
		return nil, 0, ErrInvalidList
	}

	clauses := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if opts.Done != nil {
		clauses = append(clauses, "done = ?")
		args = append(args, boolInt(*opts.Done))
	}
	if search := strings.TrimSpace(opts.Search); search != "" {
		clauses = append(clauses, "title COLLATE NOCASE LIKE ?")
		args = append(args, "%"+search+"%")
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	queryArgs := append(append([]any(nil), args...), limit, opts.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, done FROM tasks`+where+` ORDER BY id LIMIT ? OFFSET ?`,
		queryArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, total, nil
}

func (s *SQLiteStore) Get(ctx context.Context, id int64) (Task, error) {
	return getTask(ctx, s.db, id)
}

func (s *SQLiteStore) Create(ctx context.Context, rawTitle string) (Task, error) {
	title := strings.TrimSpace(rawTitle)
	if title == "" {
		return Task{}, ErrInvalidTitle
	}

	var created Task
	err := sqlitekit.RunInTransaction(ctx, s.db, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`INSERT INTO tasks (title, done) VALUES (?, ?)`,
			title, 0,
		)
		if err != nil {
			return fmt.Errorf("insert task: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("read inserted task id: %w", err)
		}
		created, err = getTask(ctx, tx, id)
		return err
	})
	if err != nil {
		return Task{}, err
	}
	return created, nil
}

func (s *SQLiteStore) Update(ctx context.Context, id int64, update Update) (Task, error) {
	if update.Title == nil && update.Done == nil {
		return Task{}, ErrNoFields
	}

	sets := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if update.Title != nil {
		title := strings.TrimSpace(*update.Title)
		if title == "" {
			return Task{}, ErrInvalidTitle
		}
		sets = append(sets, "title = ?")
		args = append(args, title)
	}
	if update.Done != nil {
		sets = append(sets, "done = ?")
		args = append(args, boolInt(*update.Done))
	}
	args = append(args, id)

	var updated Task
	err := sqlitekit.RunInTransaction(ctx, s.db, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE tasks SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
			args...,
		)
		if err != nil {
			return fmt.Errorf("update task: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read update result: %w", err)
		}
		if changed == 0 {
			return ErrNotFound
		}
		updated, err = getTask(ctx, tx, id)
		return err
	})
	if err != nil {
		return Task{}, err
	}
	return updated, nil
}

func (s *SQLiteStore) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result: %w", err)
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	err := s.db.QueryRowContext(ctx, `
SELECT
    COUNT(*),
    COALESCE(SUM(CASE WHEN done = 1 THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN done = 0 THEN 1 ELSE 0 END), 0)
FROM tasks
`).Scan(&stats.Total, &stats.Done, &stats.Open)
	if err != nil {
		return Stats{}, fmt.Errorf("read task statistics: %w", err)
	}
	return stats, nil
}

func (s *SQLiteStore) Reset(ctx context.Context, seed []SeedTask) ([]Task, error) {
	if seed == nil {
		seed = DefaultSeed
	}
	err := sqlitekit.RunInTransaction(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM tasks`); err != nil {
			return fmt.Errorf("clear tasks: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sqlite_sequence WHERE name = ?`, "tasks"); err != nil {
			return fmt.Errorf("reset task sequence: %w", err)
		}
		for _, item := range seed {
			title := strings.TrimSpace(item.Title)
			if title == "" {
				return ErrInvalidTitle
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO tasks (title, done) VALUES (?, ?)`,
				title, boolInt(item.Done),
			); err != nil {
				return fmt.Errorf("insert reset task: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	tasks, _, err := s.List(ctx, ListOptions{Limit: MaxLimit})
	return tasks, err
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type scanner interface {
	Scan(...any) error
}

func getTask(ctx context.Context, db queryRower, id int64) (Task, error) {
	task, err := scanTask(db.QueryRowContext(ctx,
		`SELECT id, title, done FROM tasks WHERE id = ?`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("get task: %w", err)
	}
	return task, nil
}

func scanTask(row scanner) (Task, error) {
	var task Task
	var done int
	if err := row.Scan(&task.ID, &task.Title, &done); err != nil {
		return Task{}, err
	}
	task.Done = done != 0
	return task, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

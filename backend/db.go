package main

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DB struct {
	conn *sql.DB
}

func NewDB(dataSourceName string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.createTables(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		completed BOOLEAN DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.conn.Exec(query)
	return err
}

func (db *DB) GetAllTasks(ctx context.Context) ([]Task, error) {
	ctx, span := GetTracer().Start(ctx, "db.GetAllTasks",
		trace.WithAttributes(attribute.String("db.operation", "select_all_tasks")))
	defer span.End()
	query := `SELECT id, title, completed, created_at FROM tasks ORDER BY created_at DESC`
	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		err := rows.Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

func (db *DB) CreateTask(ctx context.Context, title string) (*Task, error) {
	ctx, span := GetTracer().Start(ctx, "db.CreateTask",
		trace.WithAttributes(
			attribute.String("db.operation", "insert_task"),
			attribute.String("task.title", title),
		))
	defer span.End()
	query := `INSERT INTO tasks (title) VALUES (?) RETURNING id, title, completed, created_at`

	task := &Task{}
	err := db.conn.QueryRowContext(ctx, query, title).Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt)
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (db *DB) DeleteTask(ctx context.Context, id int) error {
	ctx, span := GetTracer().Start(ctx, "db.DeleteTask",
		trace.WithAttributes(
			attribute.String("db.operation", "delete_task"),
			attribute.Int("task.id", id),
		))
	defer span.End()
	query := `DELETE FROM tasks WHERE id = ?`
	result, err := db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (db *DB) CompleteTask(ctx context.Context, id int) (*Task, error) {
	ctx, span := GetTracer().Start(ctx, "db.CompleteTask",
		trace.WithAttributes(
			attribute.String("db.operation", "update_task"),
			attribute.Int("task.id", id),
		))
	defer span.End()
	query := `UPDATE tasks SET completed = 1 WHERE id = ? RETURNING id, title, completed, created_at`

	task := &Task{}
	err := db.conn.QueryRowContext(ctx, query, id).Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt)
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

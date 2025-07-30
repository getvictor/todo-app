package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/XSAM/otelsql"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

type DB struct {
	conn *sql.DB
}

func NewDB(dataSourceName string) (*DB, error) {
	// Register the otelsql wrapper for sqlite3
	driverName, err := otelsql.Register("sqlite3",
		otelsql.WithAttributes(
			semconv.DBSystemSqlite,
			attribute.String("db.name", "tasks.db"),
		),
		otelsql.WithTracerProvider(otel.GetTracerProvider()),
		otelsql.WithMeterProvider(otel.GetMeterProvider()),
		otelsql.WithSQLCommenter(true),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			// Only create spans when there's an existing parent span
			SpanFilter: func(ctx context.Context, method otelsql.Method, query string, args []driver.NamedValue) bool {
				// Check if there's a valid parent span in the context
				return trace.SpanFromContext(ctx).SpanContext().IsValid()
			},
		}),
		otelsql.WithAttributesGetter(func(ctx context.Context, method otelsql.Method, query string, args []driver.NamedValue) []attribute.KeyValue {
			// Format the query with actual values instead of placeholders
			formattedQuery := formatQueryWithArgs(query, args)
			return []attribute.KeyValue{
				attribute.String("db.statement.formatted", formattedQuery),
			}
		}),
	)
	if err != nil {
		return nil, err
	}

	// Open the database with the instrumented driver
	conn, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	// Configure connection pool for better observability
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)

	// Register database statistics metrics
	err = otelsql.RegisterDBStatsMetrics(conn,
		otelsql.WithAttributes(
			semconv.DBSystemSqlite,
			attribute.String("db.name", "tasks.db"),
		),
	)
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

	// Dummy error for demonstration purposes
	if title == "errorTest" {
		err := fmt.Errorf("simulated database error: cannot create task with title 'errorTest'")

		// Capture stack trace
		stackTrace := string(debug.Stack())

		// Record error with stack trace
		span.RecordError(err, trace.WithStackTrace(true))
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(
			attribute.String("error.type", "SimulatedError"),
			attribute.Bool("error.simulated", true),
			attribute.String("exception.stacktrace", stackTrace),
		)

		// Add an event with the stack trace for better visibility
		span.AddEvent("error.with.stacktrace",
			trace.WithAttributes(
				attribute.String("error.message", err.Error()),
				attribute.String("stack.trace", stackTrace),
			),
		)

		return nil, err
	}

	query := `INSERT INTO tasks (title) VALUES (?) RETURNING id, title, completed, created_at`

	task := &Task{}
	err := db.conn.QueryRowContext(ctx, query, title).Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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

// formatQueryWithArgs replaces SQL placeholders with actual values for better observability
func formatQueryWithArgs(query string, args []driver.NamedValue) string {
	if len(args) == 0 {
		return query
	}

	formattedQuery := query
	for i, arg := range args {
		placeholder := "?"
		if strings.Contains(query, "$") {
			// PostgreSQL style placeholders
			placeholder = fmt.Sprintf("$%d", i+1)
		}

		var value string
		if arg.Value == nil {
			value = "NULL"
		} else {
			switch v := arg.Value.(type) {
			case string:
				value = fmt.Sprintf("'%s'", v)
			case []byte:
				value = fmt.Sprintf("'%s'", string(v))
			case int64, int32, int16, int8, int:
				value = fmt.Sprintf("%d", v)
			case float64, float32:
				value = fmt.Sprintf("%f", v)
			case bool:
				if v {
					value = "TRUE"
				} else {
					value = "FALSE"
				}
			default:
				value = fmt.Sprintf("%v", v)
			}
		}

		// Replace the first occurrence of the placeholder
		formattedQuery = strings.Replace(formattedQuery, placeholder, value, 1)
	}

	return formattedQuery
}

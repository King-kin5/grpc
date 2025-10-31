package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/King-kin5/task/internal/domains"
)
// WriteRepository handles write operations for tasks
type WriteRepository struct {
	db *sql.DB
}
// NewWriteRepository creates a new write repository instance
func NewWriteRepository(db *sql.DB) *WriteRepository {
	return &WriteRepository{db: db}
}
// Save creates or updates a task in the write database
func (r *WriteRepository) Save(ctx context.Context, task *domains.Task) error {
	query := `
		INSERT INTO tasks_write (id, title, description, status, created_at, updated_at, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at,
			version = EXCLUDED.version
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		task.ID,
		task.Title,
		task.Description,
		string(task.Status),
		task.CreatedAt,
		task.UpdatedAt,
		task.Version,
	)

	if err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}

	return nil
}
// FindByID retrieves a task by its ID from the write database
func (r *WriteRepository) FindByID(ctx context.Context, id uuid.UUID) (*domains.Task, error) {
	query := `
		SELECT id, title, description, status, created_at, updated_at, version
		FROM tasks_write
		WHERE id = $1
	`
	var task domains.Task
	var status string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.Version,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find task: %w", err)
	}
	task.Status = domains.TaskStatus(status)
	return &task, nil
}
// Delete removes a task from the write database
func (r *WriteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tasks_write WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	return nil
}
// SaveEvent stores a task event in the event log
func (r *WriteRepository) SaveEvent(ctx context.Context, event *domains.TaskEvent) error {
	eventDataJSON, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	query := `
		INSERT INTO task_events (task_id, event_type, event_data, occurred_at, processed)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err = r.db.QueryRowContext(
		ctx,
		query,
		event.TaskID,
		event.EventType,
		eventDataJSON,
		event.OccurredAt,
		event.Processed,
	).Scan(&event.ID)

	if err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}
	return nil
}
// GetUnprocessedEvents retrieves unprocessed events
func (r *WriteRepository) GetUnprocessedEvents(ctx context.Context, limit int) ([]*domains.TaskEvent, error) {
	query := `
		SELECT id, task_id, event_type, event_data, occurred_at, processed
		FROM task_events
		WHERE processed = false
		ORDER BY id ASC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get unprocessed events: %w", err)
	}
	defer rows.Close()

	var events []*domains.TaskEvent

	for rows.Next() {
		var event domains.TaskEvent
		var eventDataJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.TaskID,
			&event.EventType,
			&eventDataJSON,
			&event.OccurredAt,
			&event.Processed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		err = json.Unmarshal(eventDataJSON, &event.EventData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
		}

		events = append(events, &event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}

// MarkEventProcessed marks an event as processed
func (r *WriteRepository) MarkEventProcessed(ctx context.Context, eventID int64) error {
	query := `UPDATE task_events SET processed = true WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}

	return nil
}

// BeginTx starts a new database transaction
func (r *WriteRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

// SaveWithEvent saves a task and its event in a single transaction
func (r *WriteRepository) SaveWithEvent(ctx context.Context, task *domains.Task, event *domains.TaskEvent) error {
	tx, err := r.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Save task
	taskQuery := `
		INSERT INTO tasks_write (id, title, description, status, created_at, updated_at, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at,
			version = EXCLUDED.version
	`

	_, err = tx.ExecContext(
		ctx,
		taskQuery,
		task.ID,
		task.Title,
		task.Description,
		string(task.Status),
		task.CreatedAt,
		task.UpdatedAt,
		task.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to save task in transaction: %w", err)
	}

	// Save event
	eventDataJSON, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	eventQuery := `
		INSERT INTO task_events (task_id, event_type, event_data, occurred_at, processed)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	err = tx.QueryRowContext(
		ctx,
		eventQuery,
		event.TaskID,
		event.EventType,
		eventDataJSON,
		event.OccurredAt,
		event.Processed,
	).Scan(&event.ID)
	if err != nil {
		return fmt.Errorf("failed to save event in transaction: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
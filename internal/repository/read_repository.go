package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/King-kin5/task/internal/domains"
)
// ReadRepository handles read operations for tasks
type ReadRepository struct {
	db *sql.DB
}

// NewReadRepository creates a new read repository instance
func NewReadRepository(db *sql.DB) *ReadRepository {
	return &ReadRepository{db: db}
}

// FindByID retrieves a task by its ID from the read database
func (r *ReadRepository) FindByID(ctx context.Context, id uuid.UUID) (*domains.Task, error) {
	query := `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks_read
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

// FindAll retrieves all tasks with optional filtering and pagination
func (r *ReadRepository) FindAll(ctx context.Context, status *domains.TaskStatus, page, pageSize int) ([]*domains.Task, int, error) {
	// Build query with optional status filter
	var query string
	var countQuery string
	var args []interface{}
	argIndex := 1

	baseQuery := "FROM tasks_read"
	whereClause := ""

	if status != nil {
		whereClause = fmt.Sprintf(" WHERE status = $%d", argIndex)
		args = append(args, string(*status))
		argIndex++
	}

	// Count total records
	countQuery = "SELECT COUNT(*) " + baseQuery + whereClause
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get paginated results
	query = fmt.Sprintf(`
		SELECT id, title, description, status, created_at, updated_at
		%s%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseQuery, whereClause, argIndex, argIndex+1)

	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*domains.Task

	for rows.Next() {
		var task domains.Task
		var statusStr string

		err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&statusStr,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan task: %w", err)
		}

		task.Status = domains.TaskStatus(statusStr)
		tasks = append(tasks, &task)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, total, nil
}

// Upsert creates or updates a task in the read database
func (r *ReadRepository) Upsert(ctx context.Context, task *domains.Task) error {
	query := `
		INSERT INTO tasks_read (id, title, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
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
	)

	if err != nil {
		return fmt.Errorf("failed to upsert task: %w", err)
	}

	return nil
}

// Delete removes a task from the read database
func (r *ReadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tasks_read WHERE id = $1`

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

// GetStatistics retrieves task statistics from the read database
func (r *ReadRepository) GetStatistics(ctx context.Context) (*domains.Statistics, error) {
	query := `
		SELECT 
			total_tasks,
			pending_tasks,
			in_progress_tasks,
			completed_tasks,
			cancelled_tasks,
			last_updated
		FROM task_statistics
		WHERE id = 1
	`

	var stats domains.Statistics

	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalTasks,
		&stats.PendingTasks,
		&stats.InProgressTasks,
		&stats.CompletedTasks,
		&stats.CancelledTasks,
		&stats.LastUpdated,
	)

	if err == sql.ErrNoRows {
		// Return zero statistics if not found
		return &domains.Statistics{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	return &stats, nil
}

// RefreshStatistics recalculates and updates task statistics
func (r *ReadRepository) RefreshStatistics(ctx context.Context) error {
	query := `SELECT refresh_task_statistics()`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to refresh statistics: %w", err)
	}

	return nil
}

// FindActiveTasks retrieves all active tasks (pending or in progress)
func (r *ReadRepository) FindActiveTasks(ctx context.Context) ([]*domains.Task, error) {
	query := `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks_read
		WHERE is_active = true
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find active tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*domains.Task

	for rows.Next() {
		var task domains.Task
		var status string

		err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&status,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		task.Status = domains.TaskStatus(status)
		tasks = append(tasks, &task)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// FindCompletedTasks retrieves all completed tasks
func (r *ReadRepository) FindCompletedTasks(ctx context.Context) ([]*domains.Task, error) {
	query := `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks_read
		WHERE is_completed = true
		ORDER BY updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find completed tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*domains.Task

	for rows.Next() {
		var task domains.Task
		var status string

		err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&status,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		task.Status = domains.TaskStatus(status)
		tasks = append(tasks, &task)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}
package domains

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	StatusPending    TaskStatus = "PENDING"
	StatusInProgress TaskStatus = "IN_PROGRESS"
	StatusCompleted  TaskStatus = "COMPLETED"
	StatusCancelled  TaskStatus = "CANCELLED"
)

// Validate checks if the task status is valid
func (s TaskStatus) Validate() error {
	switch s {
	case StatusPending, StatusInProgress, StatusCompleted, StatusCancelled:
		return nil
	default:
		return errors.New("invalid task status")
	}
}
type Task struct {
	ID          uuid.UUID
	Title       string
	Description string
	Status      TaskStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int
}
func NewTask(title, description string) (*Task, error) {
	if err := validateTitle(title); err != nil {
		return nil, err
	}

	now := time.Now()
	return &Task{
		ID:          uuid.New(),
		Title:       title,
		Description: description,
		Status:      StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
		Version:     1,
	}, nil
}
// UpdateStatus updates the task status with validation
func (t *Task) UpdateStatus(newStatus TaskStatus) error {
	if err := newStatus.Validate(); err != nil {
		return err
	}
	if err := t.validateStatusTransition(newStatus); err != nil {
		return err
	}
	t.Status = newStatus
	t.UpdatedAt = time.Now()
	t.Version++
	return nil
}
func (t *Task) Update(title, description string) error {
	if err := validateTitle(title); err != nil {
		return err
	}
	t.Title = title
	t.Description = description
	t.UpdatedAt = time.Now()
	t.Version++
	return nil
}

// validateStatusTransition checks if status transition is allowed
func (t *Task) validateStatusTransition(newStatus TaskStatus) error {
	// Completed tasks cannot be moved to other states
	if t.Status == StatusCompleted {
		return errors.New("cannot change status of completed task")
	}
	// Cancelled tasks cannot be moved to other states
	if t.Status == StatusCancelled {
		return errors.New("cannot change status of cancelled task")
	}
	// Cannot skip from pending to completed
	if t.Status == StatusPending && newStatus == StatusCompleted {
		return errors.New("task must be in progress before completion")
	}
	return nil
}

// validateTitle validates the task title
func validateTitle(title string) error {
	if title == "" {
		return errors.New("task title cannot be empty")
	}
	if len(title) > 255 {
		return errors.New("task title cannot exceed 255 characters")
	}
	return nil
}
func (t *Task) IsCompleted() bool {
	return t.Status == StatusCompleted
}
func (t *Task) IsActive() bool {
	return t.Status == StatusPending || t.Status == StatusInProgress
}
// TaskEvent represents a domain event
type TaskEvent struct {
	ID        int64
	TaskID    uuid.UUID
	EventType string
	EventData map[string]interface{}
	OccurredAt time.Time
	Processed bool
}

// Event types
const (
	EventTaskCreated       = "TASK_CREATED"
	EventTaskStatusUpdated = "TASK_STATUS_UPDATED"
	EventTaskUpdated       = "TASK_UPDATED"
	EventTaskDeleted       = "TASK_DELETED"
)
// NewTaskCreatedEvent creates a task created event
func NewTaskCreatedEvent(task *Task) *TaskEvent {
	return &TaskEvent{
		TaskID:    task.ID,
		EventType: EventTaskCreated,
		EventData: map[string]interface{}{
			"id":          task.ID.String(),
			"title":       task.Title,
			"description": task.Description,
			"status":      string(task.Status),
			"created_at":  task.CreatedAt,
			"updated_at":  task.UpdatedAt,
		},
		OccurredAt: time.Now(),
		Processed:  false,
	}
}
// NewTaskStatusUpdatedEvent creates a task status updated event
func NewTaskStatusUpdatedEvent(task *Task, oldStatus TaskStatus) *TaskEvent {
	return &TaskEvent{
		TaskID:    task.ID,
		EventType: EventTaskStatusUpdated,
		EventData: map[string]interface{}{
			"id":         task.ID.String(),
			"old_status": string(oldStatus),
			"new_status": string(task.Status),
			"updated_at": task.UpdatedAt,
		},
		OccurredAt: time.Now(),
		Processed:  false,
	}
}
func NewTaskDeletedEvent(taskID uuid.UUID) *TaskEvent {
	return &TaskEvent{
		TaskID:    taskID,
		EventType: EventTaskDeleted,
		EventData: map[string]interface{}{
			"id":         taskID.String(),
			"deleted_at": time.Now(),
		},
		OccurredAt: time.Now(),
		Processed:  false,
	}
}
// Statistics represents task statistics
type Statistics struct {
	TotalTasks       int
	PendingTasks     int
	InProgressTasks  int
	CompletedTasks   int
	CancelledTasks   int
	LastUpdated      time.Time
}
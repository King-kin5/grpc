package command

import (
	"context"
	"fmt"

	"github.com/King-kin5/task/internal/domains"
	"github.com/King-kin5/task/internal/repository"
	"github.com/King-kin5/task/queue"
	"github.com/google/uuid"
)

// CreateTaskCommand represents a create task command
type CreateTaskCommand struct {
	Title       string
	Description string
}

type UpdateTaskStatusCommand struct {
	TaskID    uuid.UUID
	NewStatus domains.TaskStatus
}
type DeleteTaskCommand struct {
	TaskID uuid.UUID
}
// CommandHandler handles all command operations (writes)
type CommandHandler struct{
	writeRepo*repository.WriteRepository
	publisher *queue.Publisher
}
func NewCommandHandler(writeRepo *repository.WriteRepository, publisher *queue.Publisher) *CommandHandler {
	return &CommandHandler{
		writeRepo: writeRepo,
		publisher: publisher,
	}
}
// HandleCreateTask handles the create task command
func (h *CommandHandler)HandleCreateTask(ctx context.Context,cmd *CreateTaskCommand)  (*domains.Task, error) {
	task,err:=domains.NewTask(cmd.Title,cmd.Description)
	if err != nil {
		return nil,fmt.Errorf("ffailed to create task:%n",err)
	}
	//create event
	event:=domains.NewTaskCreatedEvent(task)
	// save event
	err=h.writeRepo.SaveWithEvent(ctx,task,event)
		if err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}
	// Publish event to message queue (async)
	err = h.publisher.PublishEvent(ctx, event)
	if err != nil {
		fmt.Printf("Warning: failed to publish event: %v\n", err)
	}
	return task,nil
}

func (h *CommandHandler) HandleUpdateTaskStatus(ctx context.Context, cmd *UpdateTaskStatusCommand) (*domains.Task, error){
	// Retrieve task from write database
	task,err:=h.writeRepo.FindByID(ctx,cmd.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	// sTORE OLD STATUS for event
	oldStatus := task.Status
	// Update status
	err = task.UpdateStatus(cmd.NewStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}
	// Create event
	event := domains.NewTaskStatusUpdatedEvent(task, oldStatus)
	// Save task and event in a transaction
	err = h.writeRepo.SaveWithEvent(ctx, task, event)
	if err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}
	// Publish event to message queue (async)
	err = h.publisher.PublishEvent(ctx, event)
	if err != nil {
		fmt.Printf("Warning: failed to publish event: %v\n", err)
	}
	return task, nil
}

func (h *CommandHandler) HandleDeleteTask(ctx context.Context, cmd *DeleteTaskCommand) error {
	// Retrieve task from write database
	task,err:=h.writeRepo.FindByID(ctx,cmd.TaskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	// Create event before deletion
	event := domains.NewTaskDeletedEvent(task.ID)
	// Save event first
	err = h.writeRepo.SaveEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}
	// Delete task from write database
	err = h.writeRepo.Delete(ctx, cmd.TaskID)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	// Publish event to message queue (async)
	err = h.publisher.PublishEvent(ctx, event)
	if err != nil {
		fmt.Printf("Warning: failed to publish event: %v\n", err)
	}
	return nil
}
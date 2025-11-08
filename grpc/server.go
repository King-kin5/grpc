
package grpc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/King-kin5/task/command"
	"github.com/King-kin5/task/internal/domains"
	"github.com/King-kin5/task/query"
	taskpb "github.com/King-kin5/task/proto"
)
// TaskServer implements the gRPC TaskService
type TaskServer struct {
	taskpb.UnimplementedTaskServiceServer
	commandHandler *command.CommandHandler
	queryHandler   *query.QueryHandler
}
// NewTaskServer creates a new gRPC task server
func NewTaskServer(cmdHandler *command.CommandHandler, queryHandler *query.QueryHandler) *TaskServer {
	return &TaskServer{
		commandHandler: cmdHandler,
		queryHandler:   queryHandler,
	}
}


// CreateTask creates a new task (Command)
func (s *TaskServer) CreateTask(ctx context.Context, req *taskpb.CreateTaskRequest) (*taskpb.CreateTaskResponse, error) {
	// Validate request
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	// Create command
	cmd := &command.CreateTaskCommand{
		Title:       req.Title,
		Description: req.Description,
	}

	// Handle command
	task, err := s.commandHandler.HandleCreateTask(ctx, cmd)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create task: %v", err)
	}

	// Convert to proto message
	return &taskpb.CreateTaskResponse{
		Task:    convertTaskToProto(task),
		Message: "Task created successfully",
	}, nil
}
// UpdateTaskStatus updates a task's status (Command)
func (s *TaskServer) UpdateTaskStatus(ctx context.Context, req *taskpb.UpdateTaskStatusRequest) (*taskpb.UpdateTaskStatusResponse, error) {
	// Validate request
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	taskID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task ID")
	}

	// Convert proto status to domain status
	newStatus := convertProtoStatusToDomain(req.Status)

	// Create command
	cmd := &command.UpdateTaskStatusCommand{
		TaskID:    taskID,
		NewStatus: newStatus,
	}

	// Handle command
	task, err := s.commandHandler.HandleUpdateTaskStatus(ctx, cmd)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update task status: %v", err)
	}

	// Convert to proto message
	return &taskpb.UpdateTaskStatusResponse{
		Task:    convertTaskToProto(task),
		Message: "Task status updated successfully",
	}, nil
}
func (s *TaskServer) DeleteTask(ctx context.Context, req *taskpb.DeleteTaskRequest) (*taskpb.DeleteTaskResponse, error) {
	// Validate request
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	taskID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task ID")
	}

	// Create command
	cmd := &command.DeleteTaskCommand{
		TaskID: taskID,
	}

	// Handle command
	err = s.commandHandler.HandleDeleteTask(ctx, cmd)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete task: %v", err)
	}

	return &taskpb.DeleteTaskResponse{
		Success: true,
		Message: "Task deleted successfully",
	}, nil
}
// GetTask retrieves a single task (Query)
func (s *TaskServer) GetTask(ctx context.Context, req *taskpb.GetTaskRequest) (*taskpb.GetTaskResponse, error) {
	// Validate request
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	taskID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task ID")
	}

	// Create query
	q := &query.GetTaskQuery{
		TaskID: taskID,
	}

	// Handle query
	task, err := s.queryHandler.HandleGetTask(ctx, q)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "task not found: %v", err)
	}

	return &taskpb.GetTaskResponse{
		Task: convertTaskToProto(task),
	}, nil
}

// GetTasks retrieves multiple tasks with filtering and pagination (Query)
func (s *TaskServer) GetTasks(ctx context.Context, req *taskpb.GetTasksRequest) (*taskpb.GetTasksResponse, error) {
	// Create query
	q := &query.GetTasksQuery{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	}

	// Add status filter if provided (status 0 is PENDING, so we need special handling)
	if req.Status != 0 || (req.Status == 0 && req.Page > 0) {
		status := convertProtoStatusToDomain(req.Status)
		q.Status = &status
	}

	// Handle query
	result, err := s.queryHandler.HandleGetTasks(ctx, q)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get tasks: %v", err)
	}

	// Convert tasks to proto
	protoTasks := make([]*taskpb.Task, len(result.Tasks))
	for i, task := range result.Tasks {
		protoTasks[i] = convertTaskToProto(task)
	}

	return &taskpb.GetTasksResponse{
		Tasks:    protoTasks,
		Total:    int32(result.Total),
		Page:     int32(result.Page),
		PageSize: int32(result.PageSize),
	}, nil
}

// GetStatistics retrieves task statistics (Query)
func (s *TaskServer) GetStatistics(ctx context.Context, req *taskpb.GetStatisticsRequest) (*taskpb.GetStatisticsResponse, error) {
	// Create query
	q := &query.GetStatisticsQuery{}

	// Handle query
	stats, err := s.queryHandler.HandleGetStatistics(ctx, q)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get statistics: %v", err)
	}

	return &taskpb.GetStatisticsResponse{
		Statistics: &taskpb.Statistics{
			TotalTasks:        int32(stats.TotalTasks),
			PendingTasks:      int32(stats.PendingTasks),
			InProgressTasks:   int32(stats.InProgressTasks),
			CompletedTasks:    int32(stats.CompletedTasks),
			CancelledTasks:    int32(stats.CancelledTasks),
			LastUpdated:       timestamppb.New(stats.LastUpdated),
		},
	}, nil
}

// Helper functions

// convertTaskToProto converts a domain task to a proto task
func convertTaskToProto(task *domains.Task) *taskpb.Task {
	return &taskpb.Task{
		Id:          task.ID.String(),
		Title:       task.Title,
		Description: task.Description,
		Status:      convertDomainStatusToProto(task.Status),
		CreatedAt:   timestamppb.New(task.CreatedAt),
		UpdatedAt:   timestamppb.New(task.UpdatedAt),
	}
}

// convertDomainStatusToProto converts domain status to proto status
func convertDomainStatusToProto(status domains.TaskStatus) taskpb.TaskStatus {
	switch status {
	case domains.StatusPending:
		return taskpb.TaskStatus_PENDING
	case domains.StatusInProgress:
		return taskpb.TaskStatus_IN_PROGRESS
	case domains.StatusCompleted:
		return taskpb.TaskStatus_COMPLETED
	case domains.StatusCancelled:
		return taskpb.TaskStatus_CANCELLED
	default:
		return taskpb.TaskStatus_PENDING
	}
}

// convertProtoStatusToDomain converts proto status to domain status
func convertProtoStatusToDomain(status taskpb.TaskStatus) domains.TaskStatus {
	switch status {
	case taskpb.TaskStatus_PENDING:
		return domains.StatusPending
	case taskpb.TaskStatus_IN_PROGRESS:
		return domains.StatusInProgress
	case taskpb.TaskStatus_COMPLETED:
		return domains.StatusCompleted
	case taskpb.TaskStatus_CANCELLED:
		return domains.StatusCancelled
	default:
		return domains.StatusPending
	}
}
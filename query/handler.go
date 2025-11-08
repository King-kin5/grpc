package query


import (
	"context"
	"fmt"
	"time"

	"github.com/King-kin5/task/internal/domains"
	"github.com/King-kin5/task/internal/repository"
	"github.com/google/uuid"
)

// Query definitions

// GetTaskQuery represents a query to get a single task
type GetTaskQuery struct {
	TaskID uuid.UUID
}

// GetTasksQuery represents a query to get multiple tasks
type GetTasksQuery struct {
	Status   *domains.TaskStatus // Optional filter
	Page     int
	PageSize int
	SortBy   string // "created_at", "updated_at", "title"
	SortDesc bool   // true for descending, false for ascending
}

// GetActiveTasksQuery represents a query to get active tasks
type GetActiveTasksQuery struct {
	Limit int // Optional limit, 0 means no limit
}

// GetStatisticsQuery represents a query to get task statistics
type GetStatisticsQuery struct {
	Refresh bool // Force refresh statistics
}

// GetCompletedTasksQuery represents a query to get completed tasks
type GetCompletedTasksQuery struct {
	Limit      int       // Optional limit, 0 means no limit
	Since      time.Time // Optional: get tasks completed after this date
	Page       int       // For pagination
	PageSize   int       // For pagination
}

// GetTasksByDateRangeQuery represents a query to get tasks within a date range
type GetTasksByDateRangeQuery struct {
	StartDate time.Time
	EndDate   time.Time
	Status    *domains.TaskStatus
	Page      int
	PageSize  int
}

// GetTaskCountByStatusQuery represents a query to count tasks by status
type GetTaskCountByStatusQuery struct {
	Status domains.TaskStatus
}

// TasksResult represents the result of a tasks query with pagination
type TasksResult struct {
	Tasks    []*domains.Task
	Total    int
	Page     int
	PageSize int
	HasNext  bool // Indicates if there are more pages
	HasPrev  bool // Indicates if there are previous pages
}

// StatisticsResult represents enhanced statistics with additional metrics
type StatisticsResult struct {
	*domains.Statistics
	CompletionRate    float64 // Percentage of completed tasks
	ActiveRate        float64 // Percentage of active tasks
	AveragePerDay     float64 // Average tasks created per day
	LastTaskCreatedAt time.Time
	LastTaskUpdatedAt time.Time
}

// QueryHandler handles all query operations (reads)
type QueryHandler struct {
	readRepo *repository.ReadRepository
}

// NewQueryHandler creates a new query handler
func NewQueryHandler(readRepo *repository.ReadRepository) *QueryHandler {
	return &QueryHandler{
		readRepo: readRepo,
	}
}

// HandleGetTask handles the get task query
func (h *QueryHandler) HandleGetTask(ctx context.Context, query *GetTaskQuery) (*domains.Task, error) {
	if query.TaskID == uuid.Nil {
		return nil, fmt.Errorf("invalid task ID: cannot be nil")
	}

	task, err := h.readRepo.FindByID(ctx, query.TaskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return task, nil
}

// HandleGetTasks handles the get tasks query with filtering and pagination
func (h *QueryHandler) HandleGetTasks(ctx context.Context, query *GetTasksQuery) (*TasksResult, error) {
	// Set default pagination values
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100 // Max page size to prevent overload
	}

	// Set default sort
	if query.SortBy == "" {
		query.SortBy = "created_at"
		query.SortDesc = true // Most recent first by default
	}

	// Query with optional status filter
	tasks, total, err := h.readRepo.FindAll(ctx, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}

	// Calculate pagination metadata
	totalPages := (total + query.PageSize - 1) / query.PageSize
	hasNext := query.Page < totalPages
	hasPrev := query.Page > 1

	return &TasksResult{
		Tasks:    tasks,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		HasNext:  hasNext,
		HasPrev:  hasPrev,
	}, nil
}

// HandleGetStatistics handles the get statistics query with enhanced metrics
func (h *QueryHandler) HandleGetStatistics(ctx context.Context, query *GetStatisticsQuery) (*StatisticsResult, error) {
	// Refresh statistics if requested
	if query.Refresh {
		if err := h.readRepo.RefreshStatistics(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh statistics: %w", err)
		}
	}

	// Get base statistics
	stats, err := h.readRepo.GetStatistics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	// Calculate additional metrics
	result := &StatisticsResult{
		Statistics: stats,
	}

	// Calculate completion rate
	if stats.TotalTasks > 0 {
		result.CompletionRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
		result.ActiveRate = float64(stats.PendingTasks+stats.InProgressTasks) / float64(stats.TotalTasks) * 100
	}

	return result, nil
}

// HandleGetActiveTasks handles the get active tasks query
func (h *QueryHandler) HandleGetActiveTasks(ctx context.Context, query *GetActiveTasksQuery) ([]*domains.Task, error) {
	tasks, err := h.readRepo.FindActiveTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active tasks: %w", err)
	}

	// Apply limit if specified
	if query.Limit > 0 && len(tasks) > query.Limit {
		tasks = tasks[:query.Limit]
	}

	return tasks, nil
}

// HandleGetCompletedTasks handles the get completed tasks query with enhanced filtering
func (h *QueryHandler) HandleGetCompletedTasks(ctx context.Context, query *GetCompletedTasksQuery) (*TasksResult, error) {
	// Set default pagination
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	// Get all completed tasks
	allTasks, err := h.readRepo.FindCompletedTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get completed tasks: %w", err)
	}

	// Filter by date if specified
	var filteredTasks []*domains.Task
	if !query.Since.IsZero() {
		for _, task := range allTasks {
			if task.UpdatedAt.After(query.Since) {
				filteredTasks = append(filteredTasks, task)
			}
		}
	} else {
		filteredTasks = allTasks
	}

	// Apply pagination
	total := len(filteredTasks)
	start := (query.Page - 1) * query.PageSize
	end := start + query.PageSize

	if start >= total {
		return &TasksResult{
			Tasks:    []*domains.Task{},
			Total:    total,
			Page:     query.Page,
			PageSize: query.PageSize,
			HasNext:  false,
			HasPrev:  query.Page > 1,
		}, nil
	}

	if end > total {
		end = total
	}

	paginatedTasks := filteredTasks[start:end]

	// Calculate pagination metadata
	totalPages := (total + query.PageSize - 1) / query.PageSize
	hasNext := query.Page < totalPages
	hasPrev := query.Page > 1

	return &TasksResult{
		Tasks:    paginatedTasks,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		HasNext:  hasNext,
		HasPrev:  hasPrev,
	}, nil
}

// HandleGetTasksByDateRange handles date range queries
func (h *QueryHandler) HandleGetTasksByDateRange(ctx context.Context, query *GetTasksByDateRangeQuery) (*TasksResult, error) {
	if query.StartDate.After(query.EndDate) {
		return nil, fmt.Errorf("start date cannot be after end date")
	}

	// Set default pagination
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	tasks, total, err := h.readRepo.FindByDateRange(ctx, query.StartDate, query.EndDate, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks by date range: %w", err)
	}

	// Calculate pagination metadata
	totalPages := (total + query.PageSize - 1) / query.PageSize
	hasNext := query.Page < totalPages
	hasPrev := query.Page > 1

	return &TasksResult{
		Tasks:    tasks,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		HasNext:  hasNext,
		HasPrev:  hasPrev,
	}, nil
}

// HandleGetTaskCountByStatus handles counting tasks by specific status
func (h *QueryHandler) HandleGetTaskCountByStatus(ctx context.Context, query *GetTaskCountByStatusQuery) (int, error) {
	if err := query.Status.Validate(); err != nil {
		return 0, fmt.Errorf("invalid status: %w", err)
	}

	count, err := h.readRepo.CountByStatus(ctx, query.Status)
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks by status: %w", err)
	}

	return count, nil
}

// HandleGetRecentTasks gets the most recently created/updated tasks
func (h *QueryHandler) HandleGetRecentTasks(ctx context.Context, limit int) ([]*domains.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	tasks, err := h.readRepo.FindRecent(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent tasks: %w", err)
	}

	return tasks, nil
}

// HandleSearchTasks searches tasks by title or description
func (h *QueryHandler) HandleSearchTasks(ctx context.Context, searchTerm string, page, pageSize int) (*TasksResult, error) {
	if searchTerm == "" {
		return nil, fmt.Errorf("search term cannot be empty")
	}

	// Set default pagination
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	tasks, total, err := h.readRepo.Search(ctx, searchTerm, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to search tasks: %w", err)
	}

	// Calculate pagination metadata
	totalPages := (total + pageSize - 1) / pageSize
	hasNext := page < totalPages
	hasPrev := page > 1

	return &TasksResult{
		Tasks:    tasks,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasNext:  hasNext,
		HasPrev:  hasPrev,
	}, nil
}

// GetTasksSummary returns a quick summary of all tasks
func (h *QueryHandler) GetTasksSummary(ctx context.Context) (map[string]interface{}, error) {
	stats, err := h.readRepo.GetStatistics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	summary := map[string]interface{}{
		"total_tasks":        stats.TotalTasks,
		"pending_tasks":      stats.PendingTasks,
		"in_progress_tasks":  stats.InProgressTasks,
		"completed_tasks":    stats.CompletedTasks,
		"cancelled_tasks":    stats.CancelledTasks,
		"last_updated":       stats.LastUpdated,
	}

	// Add calculated metrics
	if stats.TotalTasks > 0 {
		summary["completion_rate"] = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
		summary["active_rate"] = float64(stats.PendingTasks+stats.InProgressTasks) / float64(stats.TotalTasks) * 100
	} else {
		summary["completion_rate"] = 0.0
		summary["active_rate"] = 0.0
	}

	return summary, nil
}
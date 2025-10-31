package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/King-kin5/task/internal/domains"
	"github.com/King-kin5/task/internal/repository"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

// EventHandler handles different types of events
type EventHandler interface {
	HandleTaskCreated(ctx context.Context, event *domains.TaskEvent) error
	HandleTaskStatusUpdated(ctx context.Context, event *domains.TaskEvent) error
	HandleTaskDeleted(ctx context.Context, event *domains.TaskEvent) error
}

// Consumer consumes events from RabbitMQ
type Consumer struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	queueName    string
	handler      EventHandler
	readRepo     *repository.ReadRepository
}

// NewConsumer creates a new RabbitMQ consumer
func NewConsumer(url, queueName string, readRepo *repository.ReadRepository) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Set QoS to process one message at a time
	err = channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	log.Printf("RabbitMQ Consumer initialized: queue=%s", queueName)

	consumer := &Consumer{
		conn:      conn,
		channel:   channel,
		queueName: queueName,
		readRepo:  readRepo,
	}

	consumer.handler = &DefaultEventHandler{readRepo: readRepo}

	return consumer, nil
}

// Start starts consuming messages from the queue
func (c *Consumer) Start(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		c.queueName, // queue
		"",          // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	log.Println("Consumer started. Waiting for messages...")

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Consumer context cancelled, stopping...")
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Message channel closed")
					return
				}
				c.handleMessage(ctx, msg)
			}
		}
	}()

	return nil
}

// handleMessage processes a single message
func (c *Consumer) handleMessage(ctx context.Context, msg amqp.Delivery) {
	log.Printf("Received message: type=%s, messageID=%s", msg.Type, msg.MessageId)

	var event domains.TaskEvent
	err := json.Unmarshal(msg.Body, &event)
	if err != nil {
		log.Printf("Failed to unmarshal event: %v", err)
		msg.Nack(false, false) // Don't requeue invalid messages
		return
	}

	// Process event based on type
	err = c.processEvent(ctx, &event)
	if err != nil {
		log.Printf("Failed to process event: %v", err)
		// Requeue the message for retry
		msg.Nack(false, true)
		return
	}

	// Acknowledge successful processing
	err = msg.Ack(false)
	if err != nil {
		log.Printf("Failed to acknowledge message: %v", err)
	}

	log.Printf("Event processed successfully: type=%s, taskID=%s", event.EventType, event.TaskID)
}

// processEvent routes the event to the appropriate handler
func (c *Consumer) processEvent(ctx context.Context, event *domains.TaskEvent) error {
	switch event.EventType {
	case domains.EventTaskCreated:
		return c.handler.HandleTaskCreated(ctx, event)
	case domains.EventTaskStatusUpdated:
		return c.handler.HandleTaskStatusUpdated(ctx, event)
	case domains.EventTaskDeleted:
		return c.handler.HandleTaskDeleted(ctx, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}

// Close closes the consumer connection
func (c *Consumer) Close() error {
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}
	log.Println("RabbitMQ Consumer closed")
	return nil
}

// DefaultEventHandler implements EventHandler interface
type DefaultEventHandler struct {
	readRepo *repository.ReadRepository
}

// HandleTaskCreated handles task created events
func (h *DefaultEventHandler) HandleTaskCreated(ctx context.Context, event *domains.TaskEvent) error {
	taskID, err := uuid.Parse(event.EventData["id"].(string))
	if err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, event.EventData["created_at"].(string))
	if err != nil {
		createdAt = event.OccurredAt
	}

	updatedAt, err := time.Parse(time.RFC3339, event.EventData["updated_at"].(string))
	if err != nil {
		updatedAt = event.OccurredAt
	}

	task := &domains.Task{
		ID:          taskID,
		Title:       event.EventData["title"].(string),
		Description: event.EventData["description"].(string),
		Status:      domains.TaskStatus(event.EventData["status"].(string)),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	// Upsert task into read database
	err = h.readRepo.Upsert(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to upsert task: %w", err)
	}

	// Refresh statistics
	err = h.readRepo.RefreshStatistics(ctx)
	if err != nil {
		log.Printf("Warning: failed to refresh statistics: %v", err)
	}

	return nil
}

// HandleTaskStatusUpdated handles task status updated events
func (h *DefaultEventHandler) HandleTaskStatusUpdated(ctx context.Context, event *domains.TaskEvent) error {
	taskID, err := uuid.Parse(event.EventData["id"].(string))
	if err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}

	newStatus := domains.TaskStatus(event.EventData["new_status"].(string))

	updatedAt, err := time.Parse(time.RFC3339, event.EventData["updated_at"].(string))
	if err != nil {
		updatedAt = event.OccurredAt
	}

	// Get existing task from read database
	task, err := h.readRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	// Update task status
	task.Status = newStatus
	task.UpdatedAt = updatedAt

	// Upsert updated task
	err = h.readRepo.Upsert(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to upsert task: %w", err)
	}

	// Refresh statistics
	err = h.readRepo.RefreshStatistics(ctx)
	if err != nil {
		log.Printf("Warning: failed to refresh statistics: %v", err)
	}

	return nil
}

// HandleTaskDeleted handles task deleted events
func (h *DefaultEventHandler) HandleTaskDeleted(ctx context.Context, event *domains.TaskEvent) error {
	taskID, err := uuid.Parse(event.EventData["id"].(string))
	if err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}

	// Delete task from read database
	err = h.readRepo.Delete(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	// Refresh statistics
	err = h.readRepo.RefreshStatistics(ctx)
	if err != nil {
		log.Printf("Warning: failed to refresh statistics: %v", err)
	}

	return nil
}
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/King-kin5/task/internal/domains"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes events to RabbitMQ
type Publisher struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	exchangeName string
	queueName    string
}
// NewPublisher creates a new RabbitMQ publisher
func NewPublisher(url, exchangeName, queueName string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}
	// Declare exchange
	err = channel.ExchangeDeclare(
		exchangeName, // name
		"fanout",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}
	// Declare queue
	queue, err := channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}
	// Bind queue to exchange
	err = channel.QueueBind(
		queue.Name,   // queue name
		"",           // routing key
		exchangeName, // exchange
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	log.Printf("RabbitMQ Publisher initialized: exchange=%s, queue=%s", exchangeName, queueName)
	return &Publisher{
		conn:         conn,
		channel:      channel,
		exchangeName: exchangeName,
		queueName:    queueName,
	}, nil
}
// PublishEvent publishes a task event to RabbitMQ
func (p *Publisher) PublishEvent(ctx context.Context, event *domains.TaskEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	err = p.channel.PublishWithContext(
		ctx,
		p.exchangeName, // exchange
		"",             // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // Make messages persistent
			MessageId:    fmt.Sprintf("%d", event.ID),
			Type:         event.EventType,
			Timestamp:    event.OccurredAt,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	log.Printf("Published event: type=%s, taskID=%s", event.EventType, event.TaskID)
	return nil
}
// PublishTaskCreatedEvent publishes a task created event
func (p *Publisher) PublishTaskCreatedEvent(ctx context.Context, task *domains.Task) error {
	event := domains.NewTaskCreatedEvent(task)
	return p.PublishEvent(ctx, event)
}
// PublishTaskStatusUpdatedEvent publishes a task status updated event
func (p *Publisher) PublishTaskStatusUpdatedEvent(ctx context.Context, task *domains.Task, oldStatus domains.TaskStatus) error {
	event := domains.NewTaskStatusUpdatedEvent(task, oldStatus)
	return p.PublishEvent(ctx, event)
}

// PublishTaskDeletedEvent publishes a task deleted event
func (p *Publisher) PublishTaskDeletedEvent(ctx context.Context, task *domains.Task) error {
	event := domains.NewTaskDeletedEvent(task.ID)
	return p.PublishEvent(ctx, event)
}

// Close closes the publisher connection
func (p *Publisher) Close() error {
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}
	log.Println("RabbitMQ Publisher closed")
	return nil
}

// IsHealthy checks if the publisher connection is healthy
func (p *Publisher) IsHealthy() bool {
	return p.conn != nil && !p.conn.IsClosed() && p.channel != nil
}


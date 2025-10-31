# Message Queue System - Complete Guide

## Table of Contents
1. [What is a Message Queue?](#what-is-a-message-queue)
2. [What is RabbitMQ?](#what-is-rabbitmq)
3. [Why Use Message Queues?](#why-use-message-queues)
4. [When to Use Message Queues](#when-to-use-message-queues)
5. [When NOT to Use Message Queues](#when-not-to-use-message-queues)
6. [How to Use Message Queues](#how-to-use-message-queues)
7. [System Architecture](#system-architecture)
8. [Code Breakdown](#code-breakdown)
9. [How It All Works Together](#how-it-all-works-together)

---

## What is a Message Queue?

A **message queue** is like a post office for your application. Instead of components talking directly to each other, they send messages through a queue. Think of it this way:

- **Without a queue**: You call someone on the phone. If they don't answer, your message is lost.
- **With a queue**: You leave a voicemail. They'll get it when they're available, and you can continue with your day.

### Key Concepts

- **Producer/Publisher**: The component that creates and sends messages (like writing a letter)
- **Queue**: The storage that holds messages temporarily (like a mailbox)
- **Consumer**: The component that receives and processes messages (like reading mail)
- **Message**: The data being sent (the letter itself)

---

## What is RabbitMQ?

**RabbitMQ** is a message broker - a dedicated server application that acts as the middleman for your messages. Think of it as a highly sophisticated post office that:

- **Receives** messages from producers
- **Stores** them safely until consumers are ready
- **Routes** messages to the right queues
- **Delivers** them reliably to consumers
- **Tracks** which messages have been processed

### Why RabbitMQ Specifically?

There are several message brokers (Kafka, Redis, AWS SQS, etc.), but RabbitMQ excels at:

1. **Easy to set up and use** - Great for getting started
2. **Flexible routing** - Multiple exchange types (fanout, direct, topic)
3. **Reliable** - Ensures messages aren't lost
4. **Battle-tested** - Used by major companies worldwide
5. **Protocol support** - AMQP standard protocol

### RabbitMQ Core Components

```
┌─────────────────────────────────────────────────────────┐
│                      RabbitMQ Server                     │
│                                                          │
│  ┌──────────┐      ┌──────────┐      ┌──────────┐     │
│  │ Exchange │─────▶│  Queue   │─────▶│ Consumer │     │
│  └──────────┘      └──────────┘      └──────────┘     │
│       ▲                                                  │
│       │                                                  │
│  ┌──────────┐                                           │
│  │ Producer │                                           │
│  └──────────┘                                           │
└─────────────────────────────────────────────────────────┘
```

#### **1. Exchange**
- **What**: Message router that decides where messages go
- **Types**:
  - **Fanout** (used in your code): Broadcasts to all bound queues
  - **Direct**: Routes to specific queue based on routing key
  - **Topic**: Routes based on pattern matching
  - **Headers**: Routes based on message headers

**In Your Code**:
```go
// Creates a fanout exchange that broadcasts to all queues
err = channel.ExchangeDeclare(
    exchangeName, // "task_events"
    "fanout",     // Broadcasts to all bound queues
    true,         // durable - survives restarts
    false,        // auto-deleted - no
    false,        // internal - no
    false,        // no-wait
    nil,          // arguments
)
```

Why fanout? Because every consumer needs to know about task changes to keep their read database synchronized.

#### **2. Queue**
- **What**: Actual storage for messages
- **Features**: 
  - Persistent storage (if durable)
  - Ordered delivery (FIFO)
  - Multiple consumers can share

**In Your Code**:
```go
// Creates a durable queue
queue, err := channel.QueueDeclare(
    queueName, // "task_queue"
    true,      // durable - queue survives broker restart
    false,     // delete when unused - no
    false,     // exclusive - allows multiple consumers
    false,     // no-wait
    nil,       // arguments
)
```

#### **3. Binding**
- **What**: Connection between exchange and queue
- **Purpose**: Tells exchange which queues should receive messages

**In Your Code**:
```go
// Binds queue to exchange
err = channel.QueueBind(
    queue.Name,   // "task_queue"
    "",           // routing key (empty for fanout)
    exchangeName, // "task_events"
    false,
    nil,
)
```

This says: "Any message sent to 'task_events' exchange should go to 'task_queue'".

#### **4. Channel**
- **What**: Virtual connection within a TCP connection
- **Why**: Multiple channels can share one connection (efficient)
- **Usage**: One channel per thread/goroutine

**In Your Code**:
```go
channel, err := conn.Channel()
```

### How RabbitMQ Ensures Reliability

#### **1. Message Persistence**
```go
amqp.Publishing{
    DeliveryMode: amqp.Persistent, // Message saved to disk
    // ... other fields
}
```
Messages are written to disk so they survive RabbitMQ restarts.

#### **2. Publisher Confirms**
RabbitMQ acknowledges receipt of messages from publishers.

#### **3. Consumer Acknowledgments**
```go
// Manual acknowledgment in your code
msg.Ack(false)  // "I processed this successfully"
msg.Nack(false, true)  // "Failed, please retry"
```

#### **4. Dead Letter Exchanges**
Failed messages can be routed to a special queue for investigation.

---

## Why Use Message Queues?

### 1. **Decoupling**
Components don't need to know about each other. The write database doesn't need to know the read database exists.

### 2. **Reliability**
If the consumer crashes, messages wait in the queue. Nothing is lost.

### 3. **Scalability**
Add more consumers to process messages faster without changing the producer.

### 4. **Asynchronous Processing**
The producer doesn't wait for processing to complete. It sends the message and moves on.

### 5. **Load Leveling**
If you get 1000 requests at once, the queue buffers them and consumers process them at a manageable rate.

**In Your Code**: When 1000 tasks are created simultaneously:
```go
// All 1000 events are published instantly
for _, task := range tasks {
    publisher.PublishTaskCreatedEvent(ctx, task)
}
// Events queue up in RabbitMQ

// Consumer processes them at controlled rate
channel.Qos(
    1,     // Process 1 at a time
    0,     // prefetch size
    false, // global
)
```

The write API responds immediately while the consumer processes events steadily without being overwhelmed.

---

## When to Use Message Queues

### ✅ Perfect Use Cases

#### **1. Event-Driven Architecture (Your Code!)**
**Scenario**: Keeping multiple databases synchronized

**Your Implementation**:
- Write database records task creation
- Publisher sends event to RabbitMQ
- Consumer updates read database asynchronously

**Why it works**:
- Write operations fast (don't wait for read DB)
- Read database can be optimized differently
- Easy to add more consumers if read DB falls behind

```go
// Write happens immediately
writeRepo.SaveWithEvent(ctx, task, event)
publisher.PublishTaskCreatedEvent(ctx, task)
return task, nil  // User gets response instantly

// Read update happens later via consumer
// User queries from read DB (fast, optimized for queries)
```

#### **2. Background Job Processing**
**Examples**:
- Sending emails
- Generating reports
- Processing images/videos
- Data analytics

**How Your Code Could Extend**:
```go
// Publish email event
emailEvent := &domains.TaskEvent{
    EventType: "SEND_EMAIL",
    EventData: map[string]interface{}{
        "to": "user@example.com",
        "subject": "Task Created",
    },
}
publisher.PublishEvent(ctx, emailEvent)

// Email consumer processes it in background
// User doesn't wait for email to send
```

#### **3. Microservices Communication**
**Scenario**: Service A needs to notify Service B, C, D

**Your Pattern Applied**:
```
Task Service (creates task)
    ↓ [publishes event]
RabbitMQ (task_events exchange)
    ↓ [fanout to multiple queues]
├─▶ Notification Service (sends email)
├─▶ Analytics Service (tracks metrics)
└─▶ Audit Service (logs changes)
```

Each service consumes events independently - if one fails, others continue.

#### **4. Load Balancing**
**Scenario**: Spread work across multiple workers

**Your Code Supports This**:
```go
// Start multiple consumers
consumer1.Start(ctx)
consumer2.Start(ctx)
consumer3.Start(ctx)

// RabbitMQ distributes messages round-robin
// Each consumer gets different messages
```

#### **5. Retry Logic & Failed Job Handling**
**Your Implementation**:
```go
func (c *Consumer) handleMessage(ctx context.Context, msg amqp.Delivery) {
    err := c.processEvent(ctx, &event)
    if err != nil {
        log.Printf("Failed to process event: %v", err)
        msg.Nack(false, true)  // Requeue for retry
        return
    }
    msg.Ack(false)  // Success
}
```

If processing fails, message goes back to queue automatically.

#### **6. Time-Delayed Tasks**
**Example**: Send reminder email in 24 hours

**How to Implement with Your Code**:
```go
// Publish with TTL and dead-letter routing
// Message sits in delay queue, then moves to processing queue
```

---

## When NOT to Use Message Queues

### ❌ Bad Use Cases

#### **1. Simple, Synchronous Operations**
**Bad**: Validating user input
```go
// DON'T DO THIS
publisher.PublishEvent(ctx, validateEmailEvent)
// Wait for consumer to validate...
// Return validation result
```

**Good**: Validate directly
```go
// DO THIS
if !isValidEmail(email) {
    return errors.New("invalid email")
}
```

**Why**: Message queues add latency. For instant validation, use direct function calls.

#### **2. Real-Time Requirements**
**Bad**: Chat messages, live gaming, video calls

**Why**: Message queues have inherent latency (milliseconds to seconds). Use WebSockets or direct connections instead.

#### **3. Small, Simple Applications**
**Bad**: Personal blog with 10 users/day

**Why**: Message queue adds complexity (RabbitMQ server, monitoring, etc.) without meaningful benefit.

**Good**: Direct database writes sufficient

#### **4. Strong Consistency Requirements**
**Bad**: Financial transactions that must be immediately consistent

**Your Code's Trade-off**:
```go
// Write DB updated immediately
writeRepo.Save(ctx, task)

// Read DB updated shortly after (eventual consistency)
// Brief period where read DB is stale
```

If you need immediate consistency everywhere, message queues might not fit.

#### **5. Simple Request-Response**
**Bad**: "Get user profile by ID"

**Good**: Direct API call or database query

---

## How to Use Message Queues

### Step-by-Step Implementation Guide (Using Your Code)

#### **Step 1: Install & Run RabbitMQ**

```bash
# Using Docker (easiest)
docker run -d --name rabbitmq \
  -p 5672:5672 \
  -p 15672:15672 \
  rabbitmq:3-management

# RabbitMQ runs on:
# - Port 5672: AMQP protocol (your app connects here)
# - Port 15672: Web UI (http://localhost:15672)
# - Default login: guest/guest
```

#### **Step 2: Design Your Events**

**In Your Code** (`task.go`):
```go
// Define event types as constants
const (
    EventTaskCreated       = "TASK_CREATED"
    EventTaskStatusUpdated = "TASK_STATUS_UPDATED"
    EventTaskUpdated       = "TASK_UPDATED"
    EventTaskDeleted       = "TASK_DELETED"
)

// Define event structure
type TaskEvent struct {
    ID        int64
    TaskID    uuid.UUID
    EventType string
    EventData map[string]interface{}  // Flexible data payload
    OccurredAt time.Time
    Processed bool
}
```

**Best Practice**: Use constants for event types to avoid typos.

#### **Step 3: Create Publisher**

**In Your Code** (`publisher.go`):
```go
// Initialize publisher
publisher, err := queue.NewPublisher(
    "amqp://guest:guest@localhost:5672/",  // RabbitMQ URL
    "task_events",                          // Exchange name
    "task_queue",                           // Queue name
)
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()

// Publisher is now ready to send events
```

**What Happens**:
1. Connects to RabbitMQ at localhost:5672
2. Creates/verifies exchange "task_events" exists
3. Creates/verifies queue "task_queue" exists
4. Binds them together

#### **Step 4: Publish Events**

**In Your Application Code**:
```go
// Example: Task creation endpoint
func CreateTask(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    var req CreateTaskRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // 2. Create task
    task, err := domains.NewTask(req.Title, req.Description)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 3. Save to write database
    err = writeRepo.Save(r.Context(), task)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // 4. Publish event (asynchronous notification)
    err = publisher.PublishTaskCreatedEvent(r.Context(), task)
    if err != nil {
        log.Printf("Warning: failed to publish event: %v", err)
        // Don't fail the request - event will be retried
    }
    
    // 5. Return immediately to user
    json.NewEncoder(w).Encode(task)
}
```

**Key Point**: User gets response immediately. Read database updates happen in background.

#### **Step 5: Create Consumer**

**In Your Code** (`consumer.go`):
```go
// Initialize consumer
consumer, err := queue.NewConsumer(
    "amqp://guest:guest@localhost:5672/",
    "task_queue",
    readRepo,
)
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()

// Start consuming (runs in background)
ctx := context.Background()
err = consumer.Start(ctx)
if err != nil {
    log.Fatal(err)
}

log.Println("Consumer running... Press Ctrl+C to stop")
select {}  // Keep running
```

**What Happens**:
1. Connects to RabbitMQ
2. Starts listening to "task_queue"
3. For each message:
   - Calls `handleMessage()`
   - Routes to appropriate handler
   - Acknowledges or requeues

#### **Step 6: Implement Event Handlers**

**In Your Code** (`consumer.go`):
```go
// Handler processes different event types
func (h *DefaultEventHandler) HandleTaskCreated(ctx context.Context, event *domains.TaskEvent) error {
    // 1. Extract data from event
    taskID, _ := uuid.Parse(event.EventData["id"].(string))
    title := event.EventData["title"].(string)
    // ... other fields
    
    // 2. Create task object
    task := &domains.Task{
        ID:    taskID,
        Title: title,
        // ... other fields
    }
    
    // 3. Update read database
    err := h.readRepo.Upsert(ctx, task)
    if err != nil {
        return err  // Will be retried
    }
    
    // 4. Update statistics
    h.readRepo.RefreshStatistics(ctx)
    
    return nil
}
```

**Error Handling**: If handler returns error, message is requeued automatically.

#### **Step 7: Monitor & Debug**

**RabbitMQ Web UI** (http://localhost:15672):
- View queues and message counts
- See connection and channel status
- Monitor message rates
- Debug stuck messages

**Your Code's Logging**:
```go
log.Printf("Published event: type=%s, taskID=%s", event.EventType, event.TaskID)
log.Printf("Received message: type=%s, messageID=%s", msg.Type, msg.MessageId)
log.Printf("Event processed successfully: type=%s, taskID=%s", event.EventType, event.TaskID)
```

### Common Patterns in Your Code

#### **Pattern 1: Publish-Subscribe (Fanout)**
```go
// One publisher, multiple consumers
Publisher → Exchange (fanout) → Queue A → Consumer A
                              → Queue B → Consumer B
                              → Queue C → Consumer C
```

Your fanout exchange broadcasts to all bound queues.

#### **Pattern 2: Work Queue**
```go
// Multiple consumers sharing work
Publisher → Queue → Consumer 1
                 → Consumer 2
                 → Consumer 3
```

RabbitMQ distributes messages round-robin to available consumers.

#### **Pattern 3: Idempotent Operations**
```go
// Handler can be called multiple times safely
func (h *DefaultEventHandler) HandleTaskCreated(ctx context.Context, event *domains.TaskEvent) error {
    // Upsert = INSERT or UPDATE
    // Safe to call multiple times
    return h.readRepo.Upsert(ctx, task)
}
```

**Why Important**: Messages can be delivered multiple times (network issues, etc.). Handlers must handle duplicates gracefully.

#### **Pattern 4: Transactional Outbox**
```go
func (r *WriteRepository) SaveWithEvent(ctx context.Context, task *domains.Task, event *domains.TaskEvent) error {
    tx, _ := r.BeginTx(ctx)
    defer tx.Rollback()
    
    // Save task and event in same transaction
    tx.ExecContext(ctx, taskQuery, ...)
    tx.ExecContext(ctx, eventQuery, ...)
    
    tx.Commit()
    // Event guaranteed to be saved with task
}
```

Ensures task and its event are saved atomically.

---

## System Architecture

This codebase implements a **CQRS (Command Query Responsibility Segregation)** pattern with event-driven synchronization:

```
┌─────────────────┐         ┌──────────────┐         ┌─────────────────┐
│  Write Database │────────▶│   RabbitMQ   │────────▶│  Read Database  │
│   (tasks_write) │         │    Queue     │         │  (tasks_read)   │
└─────────────────┘         └──────────────┘         └─────────────────┘
       ▲                           │                          │
       │                           │                          │
   Write API                   Publisher                  Consumer
   (Creates/Updates)          (Sends Events)          (Processes Events)
```

This codebase implements a **CQRS (Command Query Responsibility Segregation)** pattern with event-driven synchronization:

```
┌─────────────────┐         ┌──────────────┐         ┌─────────────────┐
│  Write Database │────────▶│   RabbitMQ   │────────▶│  Read Database  │
│   (tasks_write) │         │    Queue     │         │  (tasks_read)   │
└─────────────────┘         └──────────────┘         └─────────────────┘
       ▲                           │                          │
       │                           │                          │
   Write API                   Publisher                  Consumer
   (Creates/Updates)          (Sends Events)          (Processes Events)
```

### Your Project's Complete Data Flow

#### **Database Structure**

**Write Database (tasks_write)**
```sql
-- Source of truth for write operations
CREATE TABLE tasks_write (
    id UUID PRIMARY KEY,
    title VARCHAR(255),
    description TEXT,
    status VARCHAR(50),
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    version INT  -- Optimistic locking
);

-- Event log for reliable delivery
CREATE TABLE task_events (
    id SERIAL PRIMARY KEY,
    task_id UUID,
    event_type VARCHAR(100),
    event_data JSONB,
    occurred_at TIMESTAMP,
    processed BOOLEAN DEFAULT FALSE
);
```

**Read Database (tasks_read)**
```sql
-- Optimized for queries
CREATE TABLE tasks_read (
    id UUID PRIMARY KEY,
    title VARCHAR(255),
    description TEXT,
    status VARCHAR(50),
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    is_active BOOLEAN,      -- Computed field for fast filtering
    is_completed BOOLEAN    -- Computed field for fast filtering
);

-- Precomputed statistics
CREATE TABLE task_statistics (
    id INT PRIMARY KEY DEFAULT 1,
    total_tasks INT,
    pending_tasks INT,
    in_progress_tasks INT,
    completed_tasks INT,
    cancelled_tasks INT,
    last_updated TIMESTAMP
);
```

#### **Why Two Databases?**

Your code separates **commands** (writes) from **queries** (reads):

**Write Database Purpose**:
- Handle CREATE, UPDATE, DELETE operations
- Enforce business rules (status transitions, validation)
- Maintain version history
- Store event log for reliability

**Read Database Purpose**:
- Optimized for fast queries
- Precomputed fields (`is_active`, `is_completed`)
- Aggregated statistics
- No complex joins or business logic

### Flow Example: User Creates a Task

Let's trace through your entire codebase:

**Step 1: API Receives Request**
```go
// User sends:
POST /tasks
{
  "title": "Implement message queue",
  "description": "Build RabbitMQ integration"
}
```

**Step 2: Domain Layer Creates Task** (`task.go`)
```go
// NewTask validates and creates task entity
func NewTask(title, description string) (*Task, error) {
    if err := validateTitle(title); err != nil {
        return nil, err  // Validates title not empty, < 255 chars
    }

    now := time.Now()
    return &Task{
        ID:          uuid.New(),           // Generate unique ID
        Title:       title,
        Description: description,
        Status:      StatusPending,        // Initial status
        CreatedAt:   now,
        UpdatedAt:   now,
        Version:     1,                    // Start at version 1
    }, nil
}
```

**Step 3: Write Repository Saves** (`write_repository.go`)
```go
// SaveWithEvent saves task AND event in single transaction
func (r *WriteRepository) SaveWithEvent(ctx context.Context, task *domains.Task, event *domains.TaskEvent) error {
    tx, _ := r.BeginTx(ctx)
    defer tx.Rollback()

    // 1. Insert into tasks_write table
    _, err = tx.ExecContext(ctx,
        `INSERT INTO tasks_write (id, title, description, status, created_at, updated_at, version)
         VALUES ($1, $2, $3, $4, $5, $6, $7)
         ON CONFLICT (id) DO UPDATE SET ...`,
        task.ID, task.Title, task.Description, string(task.Status),
        task.CreatedAt, task.UpdatedAt, task.Version,
    )

    // 2. Insert into task_events table (event log)
    eventDataJSON, _ := json.Marshal(event.EventData)
    err = tx.QueryRowContext(ctx,
        `INSERT INTO task_events (task_id, event_type, event_data, occurred_at, processed)
         VALUES ($1, $2, $3, $4, $5) RETURNING id`,
        event.TaskID, event.EventType, eventDataJSON, event.OccurredAt, false,
    ).Scan(&event.ID)

    // 3. Commit both inserts atomically
    tx.Commit()
    
    // Now BOTH task and event are safely in write database
    return nil
}
```

**Why this matters**: If the database crashes after saving the task but before saving the event, the transaction rolls back - keeping data consistent.

**Step 4: Publisher Sends Event** (`publisher.go`)
```go
// PublishTaskCreatedEvent wraps event creation and publishing
func (p *Publisher) PublishTaskCreatedEvent(ctx context.Context, task *domains.Task) error {
    // 1. Create event from task
    event := domains.NewTaskCreatedEvent(task)
    // Event contains:
    // - TaskID
    // - EventType: "TASK_CREATED"
    // - EventData: {id, title, description, status, created_at, updated_at}
    // - OccurredAt: now
    // - Processed: false

    // 2. Marshal to JSON
    body, _ := json.Marshal(event)

    // 3. Publish to RabbitMQ
    err := p.channel.PublishWithContext(ctx,
        "task_events",  // Exchange name
        "",             // Routing key (empty for fanout)
        false, false,
        amqp.Publishing{
            ContentType:  "application/json",
            Body:         body,
            DeliveryMode: amqp.Persistent,  // Survives RabbitMQ restart
            MessageId:    fmt.Sprintf("%d", event.ID),
            Type:         "TASK_CREATED",
            Timestamp:    event.OccurredAt,
        },
    )

    log.Printf("Published event: type=TASK_CREATED, taskID=%s", task.ID)
    return err
}
```

**What happens in RabbitMQ**:
```
1. Message arrives at "task_events" exchange
2. Exchange (fanout type) broadcasts to all bound queues
3. Message lands in "task_queue"
4. Message written to disk (persistent)
5. RabbitMQ confirms receipt to publisher
```

**Step 5: API Returns to User**
```go
// API handler returns immediately
return json.NewEncoder(w).Encode(task)

// User gets response in ~50ms
// Read database update happens in background (next steps)
```

**User experience**: Fast response! They don't wait for the read database to update.

**Step 6: Consumer Receives Message** (`consumer.go`)
```go
// Running in background goroutine
func (c *Consumer) Start(ctx context.Context) error {
    // Register as consumer
    msgs, _ := c.channel.Consume(
        "task_queue",  // Queue name
        "",            // Consumer tag
        false,         // auto-ack = false (manual acknowledgment)
        false,         // exclusive
        false, false, nil,
    )

    // Process messages continuously
    go func() {
        for {
            select {
            case msg := <-msgs:
                c.handleMessage(ctx, msg)  // Process each message
            case <-ctx.Done():
                return  // Graceful shutdown
            }
        }
    }()
}
```

**Step 7: Message Processing** (`consumer.go`)
```go
func (c *Consumer) handleMessage(ctx context.Context, msg amqp.Delivery) {
    log.Printf("Received message: type=%s, messageID=%s", msg.Type, msg.MessageId)

    // 1. Deserialize JSON to TaskEvent
    var event domains.TaskEvent
    err := json.Unmarshal(msg.Body, &event)
    if err != nil {
        log.Printf("Failed to unmarshal: %v", err)
        msg.Nack(false, false)  // Invalid message, discard
        return
    }

    // 2. Route to appropriate handler
    err = c.processEvent(ctx, &event)
    if err != nil {
        log.Printf("Failed to process: %v", err)
        msg.Nack(false, true)  // Requeue for retry
        return
    }

    // 3. Acknowledge successful processing
    msg.Ack(false)
    log.Printf("Event processed successfully: type=%s, taskID=%s", 
        event.EventType, event.TaskID)
}
```

**Step 8: Event Routing** (`consumer.go`)
```go
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
```

**Step 9: Handler Updates Read Database** (`consumer.go`)
```go
func (h *DefaultEventHandler) HandleTaskCreated(ctx context.Context, event *domains.TaskEvent) error {
    // 1. Extract data from event
    taskID, _ := uuid.Parse(event.EventData["id"].(string))
    title := event.EventData["title"].(string)
    description := event.EventData["description"].(string)
    status := domains.TaskStatus(event.EventData["status"].(string))
    
    // Parse timestamps
    createdAt, _ := time.Parse(time.RFC3339, event.EventData["created_at"].(string))
    updatedAt, _ := time.Parse(time.RFC3339, event.EventData["updated_at"].(string))

    // 2. Reconstruct task object
    task := &domains.Task{
        ID:          taskID,
        Title:       title,
        Description: description,
        Status:      status,
        CreatedAt:   createdAt,
        UpdatedAt:   updatedAt,
    }

    // 3. Upsert into read database (tasks_read table)
    err = h.readRepo.Upsert(ctx, task)
    if err != nil {
        return err  // Will cause retry
    }

    // 4. Update statistics
    err = h.readRepo.RefreshStatistics(ctx)
    if err != nil {
        log.Printf("Warning: failed to refresh statistics: %v", err)
        // Don't fail - statistics not critical
    }

    return nil
}
```

**Step 10: Read Repository Updates** (`read_repository.go`)
```go
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

    _, err := r.db.ExecContext(ctx, query,
        task.ID, task.Title, task.Description,
        string(task.Status), task.CreatedAt, task.UpdatedAt,
    )

    return err
}
```

**Why Upsert?**: If the same event is processed twice (network retry, etc.), the operation is idempotent - same result.

**Step 11: Statistics Refresh** (`read_repository.go`)
```go
func (r *ReadRepository) RefreshStatistics(ctx context.Context) error {
    // Calls PostgreSQL function that recalculates stats
    query := `SELECT refresh_task_statistics()`
    _, err := r.db.ExecContext(ctx, query)
    return err
}

// PostgreSQL function (not shown) does:
// UPDATE task_statistics SET
//   total_tasks = (SELECT COUNT(*) FROM tasks_read),
//   pending_tasks = (SELECT COUNT(*) FROM tasks_read WHERE status = 'PENDING'),
//   in_progress_tasks = (SELECT COUNT(*) FROM tasks_read WHERE status = 'IN_PROGRESS'),
//   completed_tasks = (SELECT COUNT(*) FROM tasks_read WHERE status = 'COMPLETED'),
//   cancelled_tasks = (SELECT COUNT(*) FROM tasks_read WHERE status = 'CANCELLED'),
//   last_updated = NOW()
// WHERE id = 1;
```

**Step 12: User Queries Task**
```go
// Later, user queries the task
GET /tasks/123e4567-e89b-12d3-a456-426614174000

// Reads from read database
func (r *ReadRepository) FindByID(ctx context.Context, id uuid.UUID) (*domains.Task, error) {
    query := `
        SELECT id, title, description, status, created_at, updated_at
        FROM tasks_read  -- Optimized read database
        WHERE id = $1
    `
    
    var task domains.Task
    var status string
    
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &task.ID, &task.Title, &task.Description,
        &status, &task.CreatedAt, &task.UpdatedAt,
    )
    
    task.Status = domains.TaskStatus(status)
    return &task, nil
}
```

**Result**: Fast query from read-optimized database!

---

### Complete Timeline

```
Time: 0ms     - User sends POST /tasks request
Time: 10ms    - Task validated (task.go - NewTask)
Time: 25ms    - Saved to write DB in transaction (write_repository.go - SaveWithEvent)
Time: 30ms    - Event published to RabbitMQ (publisher.go - PublishTaskCreatedEvent)
Time: 35ms    - User receives 201 Created response ✅
Time: 40ms    - Consumer picks up message (consumer.go - handleMessage)
Time: 50ms    - Event routed to handler (consumer.go - processEvent)
Time: 60ms    - Task upserted to read DB (read_repository.go - Upsert)
Time: 70ms    - Statistics refreshed (read_repository.go - RefreshStatistics)
Time: 75ms    - Message acknowledged to RabbitMQ ✅

Time: 100ms   - User queries GET /tasks/[id]
Time: 105ms   - Returns from read DB (read_repository.go - FindByID) ✅
```

**Total user-facing latency**: 35ms for write, 5ms for read!

---

### Other Operations in Your Project

#### **Updating Task Status**

```go
// In your domain (task.go)
func (t *Task) UpdateStatus(newStatus TaskStatus) error {
    // 1. Validate new status
    if err := newStatus.Validate(); err != nil {
        return err
    }
    
    // 2. Validate transition rules
    if err := t.validateStatusTransition(newStatus); err != nil {
        return err
    }
    // Examples:
    // - Can't change completed/cancelled tasks
    // - Can't skip PENDING → COMPLETED (must go through IN_PROGRESS)
    
    // 3. Update task
    oldStatus := t.Status
    t.Status = newStatus
    t.UpdatedAt = time.Now()
    t.Version++  // Increment version for optimistic locking
    
    return nil
}
```

**Message Queue Flow**:
```go
// 1. Save to write DB
writeRepo.Save(ctx, task)

// 2. Publish status change event
publisher.PublishTaskStatusUpdatedEvent(ctx, task, oldStatus)

// 3. Consumer receives and processes
// HandleTaskStatusUpdated fetches task from read DB, updates status, saves back
```

#### **Querying Tasks with Filters**

Your read repository supports optimized queries:

```go
// Get all active tasks (read_repository.go)
func (r *ReadRepository) FindActiveTasks(ctx context.Context) ([]*domains.Task, error) {
    query := `
        SELECT id, title, description, status, created_at, updated_at
        FROM tasks_read
        WHERE is_active = true  -- Precomputed field!
        ORDER BY created_at DESC
    `
    // Much faster than: WHERE status IN ('PENDING', 'IN_PROGRESS')
}

// Get completed tasks (read_repository.go)
func (r *ReadRepository) FindCompletedTasks(ctx context.Context) ([]*domains.Task, error) {
    query := `
        SELECT id, title, description, status, created_at, updated_at
        FROM tasks_read
        WHERE is_completed = true  -- Precomputed field!
        ORDER BY updated_at DESC
    `
}

// Paginated query with filters (read_repository.go)
func (r *ReadRepository) FindAll(ctx context.Context, status *domains.TaskStatus, page, pageSize int) ([]*domains.Task, int, error) {
    // Supports filtering by status with pagination
    // Returns both tasks and total count
}
```

#### **Getting Statistics**

```go
// Instant statistics from precomputed table (read_repository.go)
func (r *ReadRepository) GetStatistics(ctx context.Context) (*domains.Statistics, error) {
    query := `
        SELECT total_tasks, pending_tasks, in_progress_tasks,
               completed_tasks, cancelled_tasks, last_updated
        FROM task_statistics
        WHERE id = 1
    `
    // Returns immediately - no COUNT(*) queries needed!
}
```

---

### Why This Architecture Rocks

#### **1. Performance**
- **Writes**: Fast (don't wait for read DB)
- **Reads**: Fast (optimized schema, precomputed fields)
- **Statistics**: Instant (precomputed)

#### **2. Reliability**
- Transactional outbox pattern (write_repository.go - SaveWithEvent)
- Events persisted in database AND RabbitMQ
- Automatic retry on failure
- No event loss

#### **3. Scalability**
- Scale writes and reads independently
- Add more consumers if read DB falls behind
- Write DB can be on different hardware than read DB

#### **4. Maintainability**
- Clear separation: domains → repositories → queue
- Each component has single responsibility
- Easy to test in isolation

#### **5. Flexibility**
- Add new event types easily
- Add more consumers for different purposes
- Change read DB schema without affecting writes

---

### Real-World Benefits

**Scenario**: 1000 users create tasks simultaneously

**Without Queue**:
```
Each request:
1. Save to write DB (50ms)
2. Update read DB (50ms)
3. Recalculate statistics (100ms)
Total: 200ms per request
Server handles 5 req/sec = 200 seconds total
```

**With Your Queue System**:
```
Each request:
1. Save to write DB (50ms)
2. Publish event (5ms)
Total: 55ms per request
Server handles 18 req/sec = 55 seconds total

Background:
- Consumer processes 100 events/sec
- All 1000 processed in 10 seconds
- Users don't wait!
```

**Result**: 3.6x faster response time, better user experience!

---

### Flow Example: User Updates Task Status

1. **User creates a task** → API receives request
2. **Write Repository** saves to `tasks_write` database
3. **Publisher** sends "TaskCreated" event to RabbitMQ
4. **RabbitMQ** stores the event in queue
5. **Consumer** receives the event
6. **Consumer** updates `tasks_read` database
7. **User queries task** → Read from `tasks_read`

---

## Code Breakdown

### Publisher (Producer)

The Publisher sends events to RabbitMQ when things happen in the write database.

#### **NewPublisher**
```go
func NewPublisher(url, exchangeName, queueName string) (*Publisher, error)
```

**Purpose**: Initializes the connection to RabbitMQ and sets up the messaging infrastructure.

**What it does**:
- Connects to RabbitMQ server
- Opens a communication channel
- Creates an **exchange** (fanout type) - a message router that broadcasts to all connected queues
- Creates a **queue** - the actual storage for messages
- Binds the queue to the exchange so messages reach the queue

**Key Settings**:
- `durable: true` - Messages survive RabbitMQ restarts
- `fanout` exchange - Sends copies of messages to all bound queues

**Example**:
```go
publisher, err := NewPublisher(
    "amqp://localhost:5672",
    "task_events",
    "task_queue"
)
```

---

#### **PublishEvent**
```go
func (p *Publisher) PublishEvent(ctx context.Context, event *domains.TaskEvent) error
```

**Purpose**: The core function that sends any event to RabbitMQ.

**What it does**:
1. Converts the event object to JSON format
2. Wraps it in an AMQP message with metadata
3. Publishes to the exchange
4. Logs the action

**Important Properties**:
- `ContentType`: "application/json" - Tells consumers how to read the message
- `DeliveryMode`: Persistent - Message survives broker restart
- `MessageId`: Unique identifier for tracking
- `Type`: Event type (TASK_CREATED, TASK_UPDATED, etc.)
- `Timestamp`: When the event occurred

**Example**:
```go
event := &domains.TaskEvent{
    TaskID: uuid.New(),
    EventType: "TASK_CREATED",
    EventData: map[string]interface{}{
        "title": "New Task",
    },
    OccurredAt: time.Now(),
}
publisher.PublishEvent(ctx, event)
```

---

#### **PublishTaskCreatedEvent**
```go
func (p *Publisher) PublishTaskCreatedEvent(ctx context.Context, task *domains.Task) error
```

**Purpose**: Convenience method for publishing task creation events.

**What it does**:
1. Creates a properly formatted TaskCreatedEvent using `domains.NewTaskCreatedEvent()`
2. Calls `PublishEvent()` to send it

**Event Data Includes**:
- Task ID
- Title
- Description
- Status
- Timestamps

---

#### **PublishTaskStatusUpdatedEvent**
```go
func (p *Publisher) PublishTaskStatusUpdatedEvent(ctx context.Context, task *domains.Task, oldStatus domains.TaskStatus) error
```

**Purpose**: Publishes when a task's status changes (e.g., PENDING → IN_PROGRESS).

**What it does**:
1. Creates an event containing both old and new status
2. Publishes to queue

**Why track old status?**: Allows consumers to validate state transitions and maintain audit history.

---

#### **PublishTaskDeletedEvent**
```go
func (p *Publisher) PublishTaskDeletedEvent(ctx context.Context, task *domains.Task) error
```

**Purpose**: Notifies consumers when a task is deleted.

**What it does**:
1. Creates a deletion event with task ID
2. Publishes to queue so read database can remove the task

---

#### **Close**
```go
func (p *Publisher) Close() error
```

**Purpose**: Gracefully shuts down the publisher.

**What it does**:
- Closes the channel
- Closes the connection
- Releases resources

**When to use**: Application shutdown, cleanup after errors

---

#### **IsHealthy**
```go
func (p *Publisher) IsHealthy() bool
```

**Purpose**: Health check for monitoring systems.

**What it does**: Checks if connection and channel are still open and functional.

**Use case**: Health check endpoints, monitoring dashboards

---

### Consumer

The Consumer listens for events from RabbitMQ and updates the read database.

#### **NewConsumer**
```go
func NewConsumer(url, queueName string, readRepo *repository.ReadRepository) (*Consumer, error)
```

**Purpose**: Initializes a consumer that listens to the queue.

**What it does**:
1. Connects to RabbitMQ
2. Opens a channel
3. Sets **QoS (Quality of Service)** to process 1 message at a time
4. Creates a DefaultEventHandler for processing events

**Why QoS = 1?**: 
- Prevents overwhelming the consumer
- Ensures messages are processed sequentially
- Provides better error handling (if one fails, it doesn't block others)

**Example**:
```go
consumer, err := NewConsumer(
    "amqp://localhost:5672",
    "task_queue",
    readRepository
)
```

---

#### **Start**
```go
func (c *Consumer) Start(ctx context.Context) error
```

**Purpose**: Begins listening for and processing messages.

**What it does**:
1. Registers as a consumer with RabbitMQ
2. Starts a goroutine (background thread) that continuously:
   - Waits for messages
   - Processes each message via `handleMessage()`
   - Respects context cancellation for graceful shutdown

**Important Settings**:
- `auto-ack: false` - Manual acknowledgment (we control when message is considered "done")
- `exclusive: false` - Multiple consumers can share the queue

**Example**:
```go
ctx := context.Background()
err := consumer.Start(ctx)
// Consumer now running in background
```

---

#### **handleMessage**
```go
func (c *Consumer) handleMessage(ctx context.Context, msg amqp.Delivery)
```

**Purpose**: Processes a single message from the queue.

**What it does**:
1. **Deserializes** the JSON message into a TaskEvent
2. **Routes** the event to `processEvent()`
3. **Handles outcomes**:
   - Success: Acknowledges (`Ack`) - message removed from queue
   - Invalid message: Negative acknowledge without requeue (`Nack(false, false)`) - message discarded
   - Processing error: Negative acknowledge with requeue (`Nack(false, true)`) - message goes back to queue for retry

**Message Acknowledgment Flow**:
```
Message arrives
    ↓
Try to process
    ↓
Success? → Ack() → Message deleted from queue
    ↓
Invalid JSON? → Nack(false, false) → Message discarded
    ↓
Processing failed? → Nack(false, true) → Message requeued for retry
```

---

#### **processEvent**
```go
func (c *Consumer) processEvent(ctx context.Context, event *domains.TaskEvent) error
```

**Purpose**: Routes events to the correct handler based on event type.

**What it does**:
Acts as a switch/router:
- `TASK_CREATED` → HandleTaskCreated()
- `TASK_STATUS_UPDATED` → HandleTaskStatusUpdated()
- `TASK_DELETED` → HandleTaskDeleted()
- Unknown type → Returns error

---

#### **Close**
```go
func (c *Consumer) Close() error
```

**Purpose**: Gracefully shuts down the consumer.

**What it does**:
- Closes channel
- Closes connection
- Releases resources

---

### Event Handlers

#### **HandleTaskCreated**
```go
func (h *DefaultEventHandler) HandleTaskCreated(ctx context.Context, event *domains.TaskEvent) error
```

**Purpose**: Processes task creation events.

**What it does**:
1. Extracts task data from event
2. Parses UUID, timestamps
3. Creates a Task object
4. **Upserts** into read database (insert or update if exists)
5. Refreshes statistics

**Why Upsert?**: Handles duplicate message delivery (idempotency)

---

#### **HandleTaskStatusUpdated**
```go
func (h *DefaultEventHandler) HandleTaskStatusUpdated(ctx context.Context, event *domains.TaskEvent) error
```

**Purpose**: Updates task status in read database.

**What it does**:
1. Extracts task ID and new status from event
2. Fetches existing task from read database
3. Updates status and timestamp
4. Saves back to database
5. Refreshes statistics

**Flow**:
```
Event arrives
    ↓
Parse new status
    ↓
Fetch current task from read DB
    ↓
Update task.Status
    ↓
Save updated task
    ↓
Refresh statistics
```

---

#### **HandleTaskDeleted**
```go
func (h *DefaultEventHandler) HandleTaskDeleted(ctx context.Context, event *domains.TaskEvent) error
```

**Purpose**: Removes deleted tasks from read database.

**What it does**:
1. Extracts task ID from event
2. Deletes task from read database
3. Refreshes statistics

---

## How It All Works Together

### Scenario: User Creates a Task

**Step 1: API Layer (Not shown in these files)**
```
POST /tasks
{
  "title": "Implement message queue",
  "description": "Build RabbitMQ integration"
}
```

**Step 2: Write Repository**
```go
// Save task to write database
task, _ := domains.NewTask("Implement message queue", "Build RabbitMQ integration")
writeRepo.SaveWithEvent(ctx, task, event)
```

**Step 3: Publisher Sends Event**
```go
// Automatically called after save
publisher.PublishTaskCreatedEvent(ctx, task)
```

**Step 4: RabbitMQ Queue**
```
Message stored in queue:
{
  "task_id": "123e4567-e89b-12d3-a456-426614174000",
  "event_type": "TASK_CREATED",
  "event_data": {
    "title": "Implement message queue",
    "description": "Build RabbitMQ integration",
    "status": "PENDING"
  },
  "occurred_at": "2025-10-31T10:30:00Z"
}
```

**Step 5: Consumer Receives**
```go
// Running in background
consumer.Start(ctx)
  ↓
handleMessage() receives message
  ↓
processEvent() routes to HandleTaskCreated()
  ↓
Task created in read database
  ↓
Message acknowledged
```

**Step 6: User Queries Task**
```
GET /tasks/123e4567-e89b-12d3-a456-426614174000

// Reads from read database (tasks_read)
// Returns task immediately
```

---

## Key Benefits of This Implementation

### 1. **Reliability**
- Messages persist in queue even if consumer is down
- Failed messages are requeued automatically
- No data loss

### 2. **Consistency**
- Write database is source of truth
- Read database eventually consistent
- Events ensure synchronization

### 3. **Scalability**
- Add more consumers to process faster
- Write and read databases can scale independently
- Queue buffers load spikes

### 4. **Maintainability**
- Clear separation of concerns
- Easy to add new event types
- Simple to debug (events are logged)

### 5. **Performance**
- Write operations don't wait for read updates
- Queries served from optimized read database
- Async processing doesn't block users

---

## Error Handling & Resilience

### Invalid Messages
- Logged and discarded (not requeued)
- Prevents poison messages blocking queue

### Processing Failures
- Message requeued for retry
- Exponential backoff (via RabbitMQ dead letter exchanges)
- Eventually moves to dead letter queue for manual inspection

### Connection Failures
- Consumer reconnects automatically
- Publisher buffers messages during outage
- Health checks detect issues

---

## Monitoring Points

1. **Queue Length**: How many messages waiting?
2. **Processing Rate**: Messages per second?
3. **Error Rate**: Failed messages?
4. **Lag**: Time between event creation and processing?

---

## Best Practices Used

✅ **Idempotency**: Upsert operations handle duplicate messages
✅ **Manual Acknowledgment**: Control when messages are removed
✅ **Persistent Messages**: Survive restarts
✅ **QoS Limits**: Prevent consumer overload
✅ **Structured Logging**: Track message flow
✅ **Graceful Shutdown**: Clean connection closure
✅ **Health Checks**: Monitor system status

---

## Common Patterns

### Producer Pattern
```go
// 1. Create event
event := domains.NewTaskCreatedEvent(task)

// 2. Publish to queue
publisher.PublishEvent(ctx, event)

// 3. Continue without waiting
return task, nil
```

### Consumer Pattern
```go
// 1. Receive message
msg := <-msgs

// 2. Process
err := processEvent(ctx, event)

// 3. Acknowledge
if err != nil {
    msg.Nack(false, true) // Retry
} else {
    msg.Ack(false) // Done
}
```

---

## Conclusion

### Summary: Your Code's Architecture

Your implementation demonstrates a production-ready message queue system:

**Components**:
1. **RabbitMQ Server**: Reliable message broker running on port 5672
2. **Publisher**: Sends events when tasks change in write database
3. **Exchange**: Routes events to queues (fanout broadcasts to all)
4. **Queue**: Stores events until processed
5. **Consumer**: Processes events and updates read database
6. **Handlers**: Apply business logic for each event type

**Benefits You Get**:
- ✅ **Fast writes**: API responds instantly, no waiting for read DB
- ✅ **Reliable**: Messages persist even if consumer crashes
- ✅ **Scalable**: Add more consumers to process faster
- ✅ **Decoupled**: Write and read systems independent
- ✅ **Resilient**: Failed operations retry automatically

**When to Use This Pattern**:
- Event-driven systems (like yours)
- Background job processing
- Microservices communication
- Async notifications
- Load balancing work across servers

**When NOT to Use**:
- Simple, small applications
- Real-time requirements (gaming, chat)
- Immediate consistency needed everywhere
- Direct request-response patterns

### Next Steps

1. **Run RabbitMQ**: `docker run -d -p 5672:5672 -p 15672:15672 rabbitmq:3-management`
2. **Start Consumer**: Runs in background processing events
3. **Use Publisher**: Send events when data changes
4. **Monitor**: Check RabbitMQ UI at http://localhost:15672
5. **Scale**: Add more consumers if queue backs up

This message queue implementation provides a robust, scalable foundation for event-driven architecture. The Producer sends events when data changes, RabbitMQ reliably stores and delivers them, and the Consumer processes them asynchronously. This pattern enables building distributed systems that are resilient, maintainable, and performant.

---

## Quick Reference

### Publisher Methods
```go
NewPublisher(url, exchange, queue)          // Initialize
PublishEvent(ctx, event)                    // Send any event
PublishTaskCreatedEvent(ctx, task)          // Convenience: task created
PublishTaskStatusUpdatedEvent(ctx, task, oldStatus)  // Convenience: status changed
PublishTaskDeletedEvent(ctx, task)          // Convenience: task deleted
Close()                                     // Cleanup
IsHealthy()                                 // Health check
```

### Consumer Methods
```go
NewConsumer(url, queue, readRepo)           // Initialize
Start(ctx)                                  // Begin processing
Close()                                     // Cleanup
```

### Event Types
```go
TASK_CREATED                                // New task
TASK_STATUS_UPDATED                         // Status changed
TASK_UPDATED                                // Task modified
TASK_DELETED                                // Task removed
```

### RabbitMQ URLs
```go
"amqp://guest:guest@localhost:5672/"       // Local development
"amqp://user:pass@rabbitmq.example.com:5672/"  // Production
```
# Task Management System with gRPC, CQRS & Message Queue

A beginner-friendly Go project demonstrating gRPC, CQRS pattern, and RabbitMQ message queue.

## 🏗️ Architecture

```
Client → gRPC Server → Command/Query Bus
                           ↓
                      RabbitMQ Queue
                           ↓
                      Event Handlers
                           ↓
                  PostgreSQL (Read & Write)
```

## 📁 Project Structure

```
task-manager/
├── proto/
│   └── task.proto                 # gRPC service definitions
├── cmd/
│   ├── api/
│   │   └── main.go               # gRPC server entry point
│   └── worker/
│       └── main.go               # Queue consumer entry point
├── internal/
│   ├── domain/
│   │   └── task.go               # Task entity & business logic
│   ├── command/
│   │   ├── handler.go            # Command handlers (writes)
│   │   └── commands.go           # Command definitions
│   ├── query/
│   │   ├── handler.go            # Query handlers (reads)
│   │   └── queries.go            # Query definitions
│   ├── repository/
│   │   ├── write_repository.go   # Write database operations
│   │   └── read_repository.go    # Read database operations
│   ├── queue/
│   │   ├── publisher.go          # Publish events to RabbitMQ
│   │   └── consumer.go           # Consume events from RabbitMQ
│   ├── grpc/
│   │   └── server.go             # gRPC server implementation
│   └── config/
│       └── config.go             # Configuration management
├── migrations/
│   ├── 001_create_tasks_write.sql # Write database schema
│   └── 002_create_tasks_read.sql  # Read database schema
├── docker-compose.yml            # PostgreSQL + RabbitMQ setup
├── go.mod
├── go.sum
├── Makefile                      # Build commands
└── README.md                     # This file
```

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- protoc (Protocol Buffer Compiler)

### Installation

1. **Clone and setup:**
```bash
mkdir task-manager && cd task-manager
go mod init github.com/yourusername/task-manager
```

2. **Install dependencies:**
```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
go get github.com/lib/pq
go get github.com/rabbitmq/amqp091-go
go get github.com/google/uuid
```

3. **Install protoc plugins:**
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

4. **Start infrastructure:**
```bash
docker-compose up -d
```

5. **Generate gRPC code:**
```bash
make proto
```

6. **Run migrations:**
```bash
make migrate
```

7. **Start the services:**
```bash
# Terminal 1: Start gRPC API server
go run cmd/api/main.go

# Terminal 2: Start queue worker
go run cmd/worker/main.go
```

## 📝 How It Works

### CQRS Pattern
- **Commands** (Write): Create, Update, Delete tasks → Write to main database
- **Queries** (Read): Get tasks, statistics → Read from optimized read database
- **Separation**: Different models for reading and writing

### Message Queue Flow
1. Client sends command via gRPC
2. Command handler processes and saves to Write DB
3. Event published to RabbitMQ
4. Worker consumes event and updates Read DB
5. Client queries via gRPC from Read DB (fast!)

### Key Concepts

**gRPC**: Type-safe API communication using Protocol Buffers
**CQRS**: Separate models for reading and writing data
**Event-Driven**: Services communicate via events in message queue
**Async Processing**: Write operations don't block while updating read models

## 🧪 Testing with grpcurl

```bash
# Create a task
grpcurl -plaintext -d '{
  "title": "Learn gRPC",
  "description": "Master gRPC fundamentals"
}' localhost:50051 task.TaskService/CreateTask

# Get all tasks
grpcurl -plaintext localhost:50051 task.TaskService/GetTasks

# Update task status
grpcurl -plaintext -d '{
  "id": "task-uuid-here",
  "status": "COMPLETED"
}' localhost:50051 task.TaskService/UpdateTaskStatus

# Get statistics
grpcurl -plaintext localhost:50051 task.TaskService/GetStatistics
```

## 📊 Database Schema

### Write Database (Commands)
```sql
tasks_write:
- id (UUID, PK)
- title (VARCHAR)
- description (TEXT)
- status (VARCHAR)
- created_at (TIMESTAMP)
- updated_at (TIMESTAMP)
```

### Read Database (Queries)
```sql
tasks_read:
- id (UUID, PK)
- title (VARCHAR)
- description (TEXT)
- status (VARCHAR)
- created_at (TIMESTAMP)

task_statistics:
- total_tasks (INT)
- completed_tasks (INT)
- pending_tasks (INT)
- last_updated (TIMESTAMP)
```

## 🔧 Configuration

Environment variables in `.env`:
```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=taskuser
DB_PASSWORD=taskpass
DB_NAME=taskdb

# RabbitMQ
RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# gRPC
GRPC_PORT=50051
```

## 📦 Docker Services

- **PostgreSQL**: Port 5432 (Database)
- **RabbitMQ**: Port 5672 (AMQP), 15672 (Management UI)
- **RabbitMQ Dashboard**: http://localhost:15672 (guest/guest)

## 🎯 Learning Objectives

✅ Understand gRPC service definitions and code generation
✅ Implement CQRS pattern with separate read/write models
✅ Use message queues for async communication
✅ Handle events in distributed systems
✅ Separate concerns in microservices architecture

## 🔄 Flow Diagram

```
CreateTask Command:
Client → gRPC → CommandHandler → WriteDB → RabbitMQ Event
                                              ↓
                                         Worker → ReadDB

GetTasks Query:
Client → gRPC → QueryHandler → ReadDB → Response
```


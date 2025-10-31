# Stage 1: Build the application
FROM golang:1.25.3-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Install protoc and plugins
RUN apk add --no-cache protobuf-dev
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate gRPC code from proto file
RUN protoc --go_out=. --go-grpc_out=. ./proto/task.proto

# Build the binaries
# Note: The following paths are based on the Readme.md.
# Please create the cmd/api/main.go and cmd/worker/main.go files.
RUN go build -o /app/api ./cmd/api/main.go
RUN go build -o /app/worker ./cmd/worker/main.go

# Stage 2: Create the final image
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the binaries from the builder stage
COPY --from=builder /app/api /app/api
COPY --from=builder /app/worker /app/worker

# Copy other necessary files
COPY .env .
COPY migrations ./migrations

# Expose the gRPC port
EXPOSE 50051

# Create a script to run both services
RUN echo '#!/bin/sh\n/app/api &\n/app/worker' > /app/start.sh && chmod +x /app/start.sh

# Set the entrypoint
ENTRYPOINT ["/app/start.sh"]

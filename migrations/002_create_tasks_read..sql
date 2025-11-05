-- Read Database Schema for CQRS Pattern
-- This table is optimized for read operations (Queries)

-- Main read-optimized tasks table
CREATE TABLE IF NOT EXISTS tasks_read (
    id UUID PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    
    -- Denormalized fields for faster queries
    is_completed BOOLEAN GENERATED ALWAYS AS (status = 'COMPLETED') STORED,
    is_active BOOLEAN GENERATED ALWAYS AS (status IN ('PENDING', 'IN_PROGRESS')) STORED
);

-- Indexes optimized for common query patterns
CREATE INDEX idx_tasks_read_status ON tasks_read(status);
CREATE INDEX idx_tasks_read_created_at ON tasks_read(created_at DESC);
CREATE INDEX idx_tasks_read_is_completed ON tasks_read(is_completed);
CREATE INDEX idx_tasks_read_is_active ON tasks_read(is_active);

-- Statistics table for dashboard/analytics
CREATE TABLE IF NOT EXISTS task_statistics (
    id INTEGER PRIMARY KEY DEFAULT 1,
    total_tasks INTEGER NOT NULL DEFAULT 0,
    pending_tasks INTEGER NOT NULL DEFAULT 0,
    in_progress_tasks INTEGER NOT NULL DEFAULT 0,
    completed_tasks INTEGER NOT NULL DEFAULT 0,
    cancelled_tasks INTEGER NOT NULL DEFAULT 0,
    last_updated TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_single_row CHECK (id = 1)
);

-- Initialize statistics table with single row
INSERT INTO task_statistics (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;

-- Materialized view for complex analytics (optional)
CREATE MATERIALIZED VIEW IF NOT EXISTS task_summary AS
SELECT 
    DATE(created_at) as date,
    status,
    COUNT(*) as count,
    AVG(EXTRACT(EPOCH FROM (updated_at - created_at))) as avg_completion_time_seconds
FROM tasks_read
GROUP BY DATE(created_at), status;

CREATE INDEX idx_task_summary_date ON task_summary(date DESC);

-- Function to refresh statistics
CREATE OR REPLACE FUNCTION refresh_task_statistics()
RETURNS void AS $$
BEGIN
    UPDATE task_statistics
    SET 
        total_tasks = (SELECT COUNT(*) FROM tasks_read),
        pending_tasks = (SELECT COUNT(*) FROM tasks_read WHERE status = 'PENDING'),
        in_progress_tasks = (SELECT COUNT(*) FROM tasks_read WHERE status = 'IN_PROGRESS'),
        completed_tasks = (SELECT COUNT(*) FROM tasks_read WHERE status = 'COMPLETED'),
        cancelled_tasks = (SELECT COUNT(*) FROM tasks_read WHERE status = 'CANCELLED'),
        last_updated = NOW()
    WHERE id = 1;
END;
$$ LANGUAGE plpgsql;
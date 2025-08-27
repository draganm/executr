-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Jobs table
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type TEXT NOT NULL,
    binary_url TEXT NOT NULL,
    binary_sha256 TEXT NOT NULL,
    arguments TEXT[],
    env_variables JSONB,
    priority TEXT CHECK (priority IN ('foreground', 'background', 'best_effort')) NOT NULL,
    status TEXT CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')) NOT NULL DEFAULT 'pending',
    executor_id TEXT,
    stdout TEXT,
    stderr TEXT,
    exit_code INTEGER,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    last_heartbeat TIMESTAMP WITH TIME ZONE
);

-- Job execution attempts history
CREATE TABLE IF NOT EXISTS job_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id) ON DELETE CASCADE NOT NULL,
    executor_id TEXT NOT NULL,
    executor_ip TEXT NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    ended_at TIMESTAMP WITH TIME ZONE,
    status TEXT CHECK (status IN ('running', 'completed', 'failed', 'timeout')) NOT NULL,
    error_message TEXT
);

-- Indexes for performance
CREATE INDEX idx_jobs_completed_at ON jobs(completed_at) WHERE status IN ('completed', 'failed', 'cancelled');
CREATE INDEX idx_jobs_heartbeat ON jobs(last_heartbeat) WHERE status = 'running';
CREATE INDEX idx_jobs_pending ON jobs(priority, created_at) WHERE status = 'pending';
CREATE INDEX idx_job_attempts_job_id ON job_attempts(job_id);
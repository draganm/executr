-- Add retry fields to jobs table
ALTER TABLE jobs
ADD COLUMN max_retries INTEGER NOT NULL DEFAULT 0,
ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0,
ADD COLUMN retry_after TIMESTAMP;
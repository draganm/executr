-- Additional indexes for query performance

-- Index for filtering jobs by type
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type) WHERE status = 'pending';

-- Index for filtering jobs by status
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);

-- Composite index for list queries with filtering
CREATE INDEX IF NOT EXISTS idx_jobs_list_filter ON jobs(status, type, priority, created_at DESC);

-- Index for executor job lookups
CREATE INDEX IF NOT EXISTS idx_jobs_executor ON jobs(executor_id) WHERE status = 'running';

-- Index for retry queries
CREATE INDEX IF NOT EXISTS idx_jobs_retry ON jobs(retry_count, retry_after) WHERE status = 'failed' AND retry_count < max_retries;

-- Index for job attempts by executor
CREATE INDEX IF NOT EXISTS idx_job_attempts_executor ON job_attempts(executor_id, started_at DESC);

-- Partial index for active executors query (simplified without NOW())
CREATE INDEX IF NOT EXISTS idx_jobs_active_executors ON jobs(executor_id, last_heartbeat) 
WHERE status = 'running';

-- Index for job age sorting (simplified - just use created_at for sorting)
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at) WHERE status = 'pending';

-- Statistics for better query planning
ANALYZE jobs;
ANALYZE job_attempts;
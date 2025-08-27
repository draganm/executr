-- Drop performance indexes
DROP INDEX IF EXISTS idx_jobs_type;
DROP INDEX IF EXISTS idx_jobs_status;
DROP INDEX IF EXISTS idx_jobs_list_filter;
DROP INDEX IF EXISTS idx_jobs_executor;
DROP INDEX IF EXISTS idx_jobs_retry;
DROP INDEX IF EXISTS idx_job_attempts_executor;
DROP INDEX IF EXISTS idx_jobs_active_executors;
DROP INDEX IF EXISTS idx_jobs_created_at;
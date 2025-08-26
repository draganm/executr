-- Drop indexes
DROP INDEX IF EXISTS idx_job_attempts_job_id;
DROP INDEX IF EXISTS idx_jobs_pending;
DROP INDEX IF EXISTS idx_jobs_heartbeat;
DROP INDEX IF EXISTS idx_jobs_completed_at;

-- Drop tables
DROP TABLE IF EXISTS job_attempts;
DROP TABLE IF EXISTS jobs;
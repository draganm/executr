-- name: GetRetriableJobs :many
SELECT * FROM jobs
WHERE status = 'failed' 
  AND retry_count < max_retries
  AND (retry_after IS NULL OR retry_after < NOW())
ORDER BY priority, created_at
LIMIT 10;

-- name: IncrementJobRetry :exec
UPDATE jobs 
SET retry_count = retry_count + 1,
    status = 'pending',
    retry_after = NOW() + INTERVAL '1 minute' * POWER(2, retry_count), -- Exponential backoff
    error_message = NULL,
    stdout = NULL,
    stderr = NULL,
    exit_code = NULL,
    started_at = NULL,
    completed_at = NULL,
    executor_id = NULL,
    last_heartbeat = NULL
WHERE id = $1
  AND status = 'failed'
  AND retry_count < max_retries;

-- name: CreateJobWithRetries :one
INSERT INTO jobs (
    type, binary_url, binary_sha256, arguments, env_variables, 
    priority, status, max_retries
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;
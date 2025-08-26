-- name: CreateJob :one
INSERT INTO jobs (
    type, binary_url, binary_sha256, arguments, env_variables, priority
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetJob :one
SELECT * FROM jobs
WHERE id = $1;

-- name: ListJobs :many
SELECT * FROM jobs
WHERE ($1::text IS NULL OR status = $1)
  AND ($2::text IS NULL OR type = $2)
  AND ($3::text IS NULL OR priority = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: UpdateJobStatus :one
UPDATE jobs
SET status = $2,
    started_at = CASE WHEN $2 = 'running' THEN COALESCE(started_at, NOW()) ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('completed', 'failed', 'cancelled') THEN NOW() ELSE completed_at END
WHERE id = $1
RETURNING *;

-- name: CancelJob :one
UPDATE jobs
SET status = 'cancelled',
    completed_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: DeleteJob :exec
DELETE FROM jobs WHERE id = $1;

-- name: ClaimNextJob :one
UPDATE jobs
SET status = 'running',
    executor_id = $1,
    started_at = NOW(),
    last_heartbeat = NOW()
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'pending'
    ORDER BY 
        CASE priority
            WHEN 'foreground' THEN 1
            WHEN 'background' THEN 2
            WHEN 'best_effort' THEN 3
        END,
        created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING *;

-- name: UpdateHeartbeat :exec
UPDATE jobs
SET last_heartbeat = NOW()
WHERE id = $1 AND executor_id = $2 AND status = 'running';

-- name: CompleteJob :one
UPDATE jobs
SET status = 'completed',
    stdout = $2,
    stderr = $3,
    exit_code = $4,
    completed_at = NOW()
WHERE id = $1 AND status = 'running'
RETURNING *;

-- name: FailJob :one
UPDATE jobs
SET status = 'failed',
    stdout = $2,
    stderr = $3,
    exit_code = $4,
    error_message = $5,
    completed_at = NOW()
WHERE id = $1 AND status = 'running'
RETURNING *;

-- name: FindStaleJobs :many
SELECT * FROM jobs
WHERE status = 'running'
  AND last_heartbeat < NOW() - INTERVAL '15 seconds';

-- name: ResetStaleJob :exec
UPDATE jobs
SET status = 'pending',
    executor_id = NULL,
    started_at = NULL,
    last_heartbeat = NULL
WHERE id = $1 AND status = 'running';

-- name: CleanupOldJobs :exec
DELETE FROM jobs
WHERE status IN ('completed', 'failed', 'cancelled')
  AND completed_at < NOW() - $1::interval;
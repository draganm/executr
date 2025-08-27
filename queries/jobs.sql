-- name: CreateJob :one
INSERT INTO jobs (
    type, binary_url, binary_sha256, arguments, env_variables,
    priority, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetJob :one
SELECT * FROM jobs WHERE id = $1;

-- name: ListJobs :many
SELECT * FROM jobs
WHERE ($1::text IS NULL OR status = $1)
  AND ($2::text IS NULL OR type = $2)
  AND ($3::text IS NULL OR priority = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: UpdateJob :exec
UPDATE jobs SET
    status = COALESCE($2, status),
    executor_id = COALESCE($3, executor_id),
    stdout = COALESCE($4, stdout),
    stderr = COALESCE($5, stderr),
    exit_code = COALESCE($6, exit_code),
    error_message = COALESCE($7, error_message),
    started_at = COALESCE($8, started_at),
    completed_at = COALESCE($9, completed_at),
    last_heartbeat = COALESCE($10, last_heartbeat)
WHERE id = $1;

-- name: DeleteJob :exec
DELETE FROM jobs WHERE id = $1;

-- name: ClaimNextJob :one
UPDATE jobs SET
    status = 'running',
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
        created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING *;

-- name: UpdateJobHeartbeat :exec
UPDATE jobs SET
    last_heartbeat = NOW()
WHERE id = $1 AND executor_id = $2 AND status = 'running';

-- name: FindStaleJobs :many
SELECT * FROM jobs
WHERE status = 'running'
  AND last_heartbeat < NOW() - INTERVAL '1 second' * CAST($1 AS INTEGER);

-- name: CancelJob :exec
UPDATE jobs SET
    status = 'cancelled',
    completed_at = NOW(),
    error_message = 'Job cancelled by user'
WHERE id = $1 AND status = 'pending';

-- name: CleanupOldJobs :exec
DELETE FROM jobs
WHERE status IN ('completed', 'failed', 'cancelled')
  AND completed_at < NOW() - INTERVAL '1 second' * CAST($1 AS INTEGER);

-- name: CompleteJob :exec
UPDATE jobs SET
    status = 'completed',
    stdout = $2,
    stderr = $3,
    exit_code = $4,
    completed_at = NOW()
WHERE id = $1 AND executor_id = $5 AND status = 'running';

-- name: FailJob :exec
UPDATE jobs SET
    status = 'failed',
    error_message = $2,
    stdout = $3,
    stderr = $4,
    exit_code = $5,
    completed_at = NOW()
WHERE id = $1 AND executor_id = $6 AND status = 'running';

-- name: ResetStaleJob :exec
UPDATE jobs SET
    status = 'pending',
    executor_id = NULL,
    started_at = NULL,
    last_heartbeat = NULL
WHERE id = $1 AND status = 'running';
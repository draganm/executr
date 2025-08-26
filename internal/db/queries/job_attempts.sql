-- name: RecordJobAttempt :one
INSERT INTO job_attempts (
    job_id, executor_id, executor_ip, status
) VALUES (
    $1, $2, $3, 'running'
)
RETURNING *;

-- name: UpdateJobAttempt :exec
UPDATE job_attempts
SET ended_at = NOW(),
    status = $2,
    error_message = $3
WHERE job_id = $1
  AND executor_id = $4
  AND ended_at IS NULL;

-- name: GetJobAttempts :many
SELECT * FROM job_attempts
WHERE job_id = $1
ORDER BY started_at DESC;

-- name: GetLatestJobAttempt :one
SELECT * FROM job_attempts
WHERE job_id = $1
ORDER BY started_at DESC
LIMIT 1;

-- name: CountJobAttempts :one
SELECT COUNT(*) as attempt_count
FROM job_attempts
WHERE job_id = $1;
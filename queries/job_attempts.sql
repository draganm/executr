-- name: CreateJobAttempt :one
INSERT INTO job_attempts (
    job_id, executor_id, executor_ip, status
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: UpdateJobAttempt :exec
UPDATE job_attempts SET
    ended_at = NOW(),
    status = $2,
    error_message = $3
WHERE id = $1;

-- name: GetJobAttempts :many
SELECT * FROM job_attempts
WHERE job_id = $1
ORDER BY started_at DESC;

-- name: GetLatestJobAttempt :one
SELECT * FROM job_attempts
WHERE job_id = $1
ORDER BY started_at DESC
LIMIT 1;
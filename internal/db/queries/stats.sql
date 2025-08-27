-- name: CountJobsByStatus :many
SELECT status, COUNT(*) as count
FROM jobs
GROUP BY status;

-- name: CountPendingJobsByPriority :many
SELECT priority, COUNT(*) as count
FROM jobs
WHERE status = 'pending'
GROUP BY priority;

-- name: GetActiveExecutors :many
SELECT DISTINCT 
    j.executor_id,
    j.id as job_id,
    j.type as job_type,
    j.last_heartbeat,
    (SELECT COUNT(*) FROM jobs WHERE executor_id = j.executor_id AND status = 'completed') as jobs_completed
FROM jobs j
WHERE j.status = 'running' 
   AND j.last_heartbeat > NOW() - INTERVAL '30 seconds'
ORDER BY j.last_heartbeat DESC;

-- name: GetJobRetryCount :one
SELECT COUNT(*) as retry_count
FROM job_attempts
WHERE job_id = $1;
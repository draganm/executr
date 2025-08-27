# API Documentation

## Base URL

```
http://<server>:8080/api/v1
```

## Authentication

Currently, no authentication is required. This should be added for production deployments.

## Endpoints

### Health Check

Check server health and database connectivity.

```http
GET /api/v1/health
```

**Response:**
```json
{
  "status": "healthy",
  "database": "connected"
}
```

### Metrics

Prometheus-compatible metrics endpoint.

```http
GET /api/v1/metrics
```

**Response:** Prometheus text format metrics

### Submit Job

Submit a new job to the queue.

```http
POST /api/v1/jobs
```

**Request Body:**
```json
{
  "type": "data-processing",
  "binary_url": "https://example.com/binary",
  "binary_sha256": "abc123...",
  "arguments": ["arg1", "arg2"],
  "env_variables": {
    "KEY1": "value1",
    "KEY2": "value2"
  },
  "priority": "background"
}
```

**Fields:**
- `type` (string, required): Job type identifier (no spaces)
- `binary_url` (string, required): URL to download executable binary
- `binary_sha256` (string, required): SHA256 hash of the binary
- `arguments` (array, optional): Command-line arguments
- `env_variables` (object, optional): Environment variables
- `priority` (string, required): One of `foreground`, `background`, `best_effort`

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "data-processing",
  "status": "pending",
  "created_at": "2024-01-01T12:00:00Z"
}
```

### List Jobs

List jobs with optional filtering.

```http
GET /api/v1/jobs?status=pending&type=data-processing&priority=background&limit=10&offset=0
```

**Query Parameters:**
- `status` (optional): Filter by status (pending, running, completed, failed, cancelled)
- `type` (optional): Filter by job type
- `priority` (optional): Filter by priority
- `limit` (optional, default: 100): Maximum number of results
- `offset` (optional, default: 0): Pagination offset

**Response:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "data-processing",
    "status": "running",
    "priority": "background",
    "executor_id": "worker-1-abc123",
    "created_at": "2024-01-01T12:00:00Z",
    "started_at": "2024-01-01T12:01:00Z"
  }
]
```

### Get Job Details

Get detailed information about a specific job.

```http
GET /api/v1/jobs/{id}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "data-processing",
  "binary_url": "https://example.com/binary",
  "binary_sha256": "abc123...",
  "arguments": ["arg1", "arg2"],
  "env_variables": {"KEY1": "value1"},
  "priority": "background",
  "status": "completed",
  "executor_id": "worker-1-abc123",
  "stdout": "Job output...",
  "stderr": "",
  "exit_code": 0,
  "created_at": "2024-01-01T12:00:00Z",
  "started_at": "2024-01-01T12:01:00Z",
  "completed_at": "2024-01-01T12:02:00Z",
  "attempts": [
    {
      "executor_id": "worker-1-abc123",
      "executor_ip": "192.168.1.100",
      "started_at": "2024-01-01T12:01:00Z",
      "ended_at": "2024-01-01T12:02:00Z",
      "status": "completed"
    }
  ]
}
```

### Cancel Job

Cancel a pending job.

```http
DELETE /api/v1/jobs/{id}
```

**Response:**
- `200 OK`: Job cancelled successfully
- `400 Bad Request`: Job is not in pending state
- `404 Not Found`: Job not found

### Claim Job (Executor)

Executor endpoint to claim the next available job.

```http
POST /api/v1/jobs/claim
```

**Request Body:**
```json
{
  "executor_id": "worker-1-abc123",
  "executor_ip": "192.168.1.100"
}
```

**Response:**
- `200 OK`: Returns job details (same as GET /api/v1/jobs/{id})
- `204 No Content`: No jobs available

### Update Heartbeat (Executor)

Update heartbeat for a running job.

```http
PUT /api/v1/jobs/{id}/heartbeat
```

**Request Body:**
```json
{
  "executor_id": "worker-1-abc123"
}
```

**Response:**
- `200 OK`: Heartbeat updated
- `404 Not Found`: Job not found or not running

### Complete Job (Executor)

Mark a job as completed with results.

```http
PUT /api/v1/jobs/{id}/complete
```

**Request Body:**
```json
{
  "executor_id": "worker-1-abc123",
  "stdout": "Job output...",
  "stderr": "",
  "exit_code": 0
}
```

**Note:** stdout and stderr are automatically truncated to 1MB each.

**Response:**
- `200 OK`: Job marked as completed
- `400 Bad Request`: Invalid state transition
- `404 Not Found`: Job not found

### Fail Job (Executor)

Mark a job as failed with error details.

```http
PUT /api/v1/jobs/{id}/fail
```

**Request Body:**
```json
{
  "executor_id": "worker-1-abc123",
  "stdout": "Partial output...",
  "stderr": "Error messages...",
  "exit_code": 1,
  "error_message": "Binary execution failed"
}
```

**Response:**
- `200 OK`: Job marked as failed
- `400 Bad Request`: Invalid state transition
- `404 Not Found`: Job not found

## Admin Endpoints

### Statistics

Get system-wide statistics.

```http
GET /api/v1/admin/stats
```

**Response:**
```json
{
  "jobs_by_status": {
    "pending": 10,
    "running": 5,
    "completed": 100,
    "failed": 2,
    "cancelled": 1
  },
  "pending_by_priority": {
    "foreground": 2,
    "background": 5,
    "best_effort": 3
  },
  "active_executors": 3
}
```

### Active Executors

List currently active executors.

```http
GET /api/v1/admin/executors
```

**Response:**
```json
[
  {
    "executor_id": "worker-1-abc123",
    "current_job_id": "550e8400-e29b-41d4-a716-446655440000",
    "job_type": "data-processing",
    "last_heartbeat": "2024-01-01T12:01:30Z",
    "jobs_completed": 42
  }
]
```

## Bulk Operations

### Bulk Submit

Submit multiple jobs in a single request.

```http
POST /api/v1/jobs/bulk
```

**Request Body:**
```json
[
  {
    "type": "job-1",
    "binary_url": "https://example.com/binary1",
    "binary_sha256": "abc123...",
    "priority": "background"
  },
  {
    "type": "job-2",
    "binary_url": "https://example.com/binary2",
    "binary_sha256": "def456...",
    "priority": "foreground"
  }
]
```

**Response:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending"
  },
  {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "status": "pending"
  }
]
```

### Bulk Cancel

Cancel multiple pending jobs.

```http
POST /api/v1/jobs/bulk/cancel
```

**Request Body:**
```json
{
  "job_ids": [
    "550e8400-e29b-41d4-a716-446655440000",
    "660e8400-e29b-41d4-a716-446655440001"
  ]
}
```

**Response:**
```json
{
  "cancelled": 2,
  "failed": 0,
  "errors": []
}
```

## Error Responses

All endpoints return errors in the following format:

```json
{
  "error": "Human-readable error message",
  "context": {
    "field": "additional context",
    "job_id": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

## HTTP Status Codes

- `200 OK`: Request succeeded
- `201 Created`: Resource created successfully
- `204 No Content`: Request succeeded with no content to return
- `400 Bad Request`: Invalid request parameters or state
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

## Rate Limiting

Currently no rate limiting is implemented. Consider adding for production deployments.

## Pagination

List endpoints support pagination using `limit` and `offset` parameters:

- `limit`: Maximum number of results (default: 100, max: 1000)
- `offset`: Number of results to skip (default: 0)

Example:
```
GET /api/v1/jobs?limit=20&offset=40
```

## Timestamps

All timestamps are in UTC and use ISO 8601 format:
```
2024-01-01T12:00:00Z
```
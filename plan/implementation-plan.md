## Executr Implementation - Step by Step Plan

### Phase 1: Project Setup and Dependencies
1. **Initialize Go module structure**
   - Create directory structure (internal/, pkg/, cmd/, e2e/)
   - Update go.mod with required dependencies
   - Update flake.nix with PostgreSQL and required tools

2. **Add core dependencies**
   - `github.com/jackc/pgx/v5` - PostgreSQL driver
   - `github.com/golang-migrate/migrate/v4` - Database migrations
   - `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` - Testing
   - `github.com/google/uuid` - UUID generation

3. **Setup sqlc configuration**
   - Create sqlc.yaml configuration file
   - Setup queries directory structure

### Phase 2: Database Layer
1. **Create database schema migrations**
   - Create migrations directory
   - Write initial migration for jobs table with all fields
   - Add job_attempts table for execution history
   - Add required indexes for performance
   - Migrations run automatically on server startup

2. **Write sqlc queries**
   - Job CRUD operations (create, get, list, update, delete)
   - Claim next available job atomically (priority order, then oldest first, set status to running)
   - Record job attempt with executor IP
   - Heartbeat update query
   - Find stale jobs query
   - Cancel job query (with status validation)
   - Cleanup old jobs query
   - Get job execution history

3. **Generate database code**
   - Run sqlc generate
   - Create database connection helper
   - Use pgx default pool settings

### Phase 3: Core Models and Shared Code
1. **Define core data structures**
   - Job model with all fields
   - JobResult model (stdout, stderr, exit code)
   - Priority enum (foreground, background, best_effort)
   - Status enum (pending, running, completed, failed, cancelled)
   - Job type: string without spaces (for filtering)

2. **Create shared utilities**
   - SHA256 calculation helper
   - HTTP client with retry logic
   - Binary downloader with progress

### Phase 4: Server Implementation
1. **Create server command structure**
   - Add server subcommand to CLI
   - Parse configuration from flags and env vars
   - Setup structured logging with slog

2. **Implement database connection and migrations**
   - Connect to PostgreSQL with pgx default pool settings
   - Implement automatic reconnection on connection loss
   - Run migrations automatically on startup
   - Handle migration failures gracefully
   - No in-memory state during DB outages (all operations fail)

3. **Build REST API handlers**
   - POST /api/v1/jobs - Submit job
   - GET /api/v1/jobs - List jobs
   - GET /api/v1/jobs/{id} - Get job details with execution history
   - DELETE /api/v1/jobs/{id} - Cancel job
   - POST /api/v1/jobs/claim - Claim next available job (returns job details or empty if none available)
   - PUT /api/v1/jobs/{id}/heartbeat - Update heartbeat
   - PUT /api/v1/jobs/{id}/complete - Mark completed
   - PUT /api/v1/jobs/{id}/fail - Mark failed

4. **Add background workers**
   - Heartbeat monitor (check every 5 seconds)
   - Job cleaner (run every hour, delete old jobs)
   - Graceful shutdown using signal.NotifyContext and context propagation

### Phase 5: Client Package
1. **Create client library structure**
   - Define client interface
   - Implement HTTP client with base URL

2. **Implement client methods**
   - SubmitJob(job) (jobID, error)
   - GetJob(jobID) (job, error)
   - ListJobs(filter) ([]job, error)
   - CancelJob(jobID) error
   - ClaimNextJob(executorID, executorIP) (job, error) - returns next available job or nil
   - Heartbeat(jobID, executorID) error
   - CompleteJob(jobID, result) error
   - FailJob(jobID, result) error

3. **Add client utilities**
   - Retry logic for transient failures
   - Error handling and types
   - Request/response serialization

### Phase 6: Executor Implementation
1. **Create executor command structure**
   - Add executor subcommand to CLI
   - Parse configuration from flags and env vars
   - Generate executor ID at startup: "{name}-{unique-id}" (e.g., "worker1-abc123")
   - Name provided via --name flag, unique ID generated automatically

2. **Implement binary cache**
   - Create cache directory structure
   - SHA256 verification before each execution
   - Cache keyed by SHA256 only (content-addressable)
   - Download if not in cache or SHA256 mismatch
   - Set execute permissions (chmod +x) on downloaded binaries
   - Fail job if SHA256 doesn't match after download
   - LRU eviction when cache size exceeds limit (default 400MB)
   - No download size or timeout limits

3. **Build job execution engine**
   - Support concurrent job execution (configurable max jobs)
   - Poll server for next available job via claim endpoint (one at a time)
   - Server decides which job to return based on priority/age
   - Create temporary working directory for received job
   - Verify cached binary SHA256 or download
   - Execute binary from cache location with job directory as working directory
   - Pass arguments as separate args (exec.Command(bin, args...))
   - Replace environment completely with job's env variables (no merging)
   - Capture stdout, stderr, exit code
   - Truncate output if > 1MB (keep first 500 lines + last lines that fit)
   - Handle execution errors
   - Stop claiming jobs after 1 minute of network failure
   - Clean up job directory after completion (best effort)
   - Graceful shutdown: let running jobs complete on shutdown signal

4. **Add heartbeat mechanism**
   - Start heartbeat goroutine when job claimed
   - Send heartbeat every 5 seconds
   - Stop on job completion/failure

5. **Implement job cleanup**
   - Create isolated working directory for each job under configurable root
   - Job directory path: {work-dir}/{job-id}/
   - Set job directory as current directory for binary execution
   - Clean up directory after job completion (best effort)
   - On executor startup, remove all subdirectories of work-dir (best effort)
   - Log cleanup failures but don't fail the job

### Phase 7: Submitter Implementation
1. **Create submit command**
   - Add submit subcommand to CLI
   - Parse job parameters from flags/env
   - Stream download binary to calculate SHA256 in-place if not provided
   - No temporary file storage - calculate SHA256 while streaming
   - Submit job to server
   - Display job ID

2. **Create status command**
   - Add status subcommand
   - Query job by ID
   - Format output (JSON or table)
   - Show execution results if available

3. **Create cancel command**
   - Add cancel subcommand
   - Send cancel request for job ID
   - Handle error if job not cancellable

### Phase 8: Testing Infrastructure
1. **Setup E2E test framework**
   - Configure Ginkgo test suite
   - Create PostgreSQL test container setup
   - Create test HTTP server for binaries

2. **Create test binaries**
   - Simple success binary
   - Binary that fails with specific exit code
   - Long-running binary for timeout tests
   - Binary that outputs to stdout/stderr

3. **Implement core E2E tests**
   - Job submission and execution flow
   - Binary caching verification
   - Priority-based execution
   - Job cancellation
   - Heartbeat timeout and recovery
   - Multiple executor coordination (different names)
   - Verify executor ID format ("{name}-{suffix}")
   - Job working directory cleanup verification
   - Executor restart with orphaned directory cleanup

### Phase 9: Advanced Features
1. **Add monitoring and metrics**
   - Job execution metrics
   - Executor health tracking
   - Performance monitoring

2. **Implement security features**
   - Binary signature verification
   - Executor sandboxing considerations

3. **Add operational tools**
   - Job retry mechanism
   - Bulk job operations
   - Admin endpoints for debugging

### Phase 10: Documentation and Polish
1. **Write documentation**
   - API documentation
   - Deployment guide
   - Configuration reference

2. **Add examples**
   - Example job submissions
   - Example executor configurations
   - Docker Compose setup for testing

3. **Performance optimization**
   - Database query optimization
   - Connection pooling tuning
   - Binary cache optimization

## Architecture Overview

### System Components

#### Server
- REST API mediator between jobs and executors
- PostgreSQL for persistent storage
- Job queue management by priority
- Heartbeat monitoring for job recovery
- Automatic cleanup of old jobs

#### Executor
- Worker process that executes jobs
- Unique ID generated at startup: "{name}-{unique-suffix}"
- Binary cache with SHA256 verification
- Heartbeat sender while running jobs
- Stdout/stderr/exit code capture
- Configurable cache directory
- Isolated working directory per job with automatic cleanup
- Cleanup of orphaned job directories on startup

#### Submitter
- CLI tool for job management
- Automatic SHA256 calculation from binary URL
- Job submission, status query, cancellation
- Multiple output formats (JSON, table)

#### Client Package
- Go library for programmatic API access
- Full API coverage with type safety
- Built-in retry logic

### Configuration

All components support both environment variables and command-line flags.

#### Server Configuration
- `--db-url` / `EXECUTR_DB_URL` - PostgreSQL connection string
- `--port` / `EXECUTR_PORT` - Server listen port (default: 8080)
- `--cleanup-interval` / `EXECUTR_CLEANUP_INTERVAL` - Cleanup frequency (default: 1h)
- `--job-retention` / `EXECUTR_JOB_RETENTION` - Keep completed jobs duration (default: 48h)
- `--heartbeat-timeout` / `EXECUTR_HEARTBEAT_TIMEOUT` - Stale job timeout (default: 15s)
- `--log-level` / `EXECUTR_LOG_LEVEL` - Log level (debug/info/warn/error)

#### Executor Configuration
- `--server-url` / `EXECUTR_SERVER_URL` - Server API endpoint
- `--name` / `EXECUTR_NAME` - Executor name (required, used as prefix for executor ID)
- `--cache-dir` / `EXECUTR_CACHE_DIR` - Binary cache directory (default: ~/.executr/cache)
- `--work-dir` / `EXECUTR_WORK_DIR` - Root directory for job working directories (default: /tmp/executr-jobs)
- `--max-jobs` / `EXECUTR_MAX_JOBS` - Maximum concurrent jobs (default: 1)
- `--poll-interval` / `EXECUTR_POLL_INTERVAL` - Job polling frequency (default: 5s)
- `--max-cache-size` / `EXECUTR_MAX_CACHE_SIZE` - Maximum cache size in MB (default: 400)
- `--heartbeat-interval` / `EXECUTR_HEARTBEAT_INTERVAL` - Heartbeat frequency (default: 5s)
- `--network-timeout` / `EXECUTR_NETWORK_TIMEOUT` - Stop claiming after network failure (default: 1m)
- `--log-level` / `EXECUTR_LOG_LEVEL` - Log level

#### Submitter Configuration
- `--server-url` / `EXECUTR_SERVER_URL` - Server API endpoint
- `--binary-url` / `EXECUTR_BINARY_URL` - Binary download URL (required for submit)
- `--binary-sha256` / `EXECUTR_BINARY_SHA256` - Binary SHA256 (optional, auto-calculated)
- `--args` / `EXECUTR_ARGS` - Arguments (comma-separated)
- `--env` / `EXECUTR_ENV` - Environment variables (KEY=VALUE,...)
- `--type` / `EXECUTR_TYPE` - Job type (informational)
- `--priority` / `EXECUTR_PRIORITY` - Priority (foreground/background/best_effort)
- `--output` / `EXECUTR_OUTPUT` - Output format (json/table)

### Database Schema

```sql
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type TEXT NOT NULL,
    binary_url TEXT NOT NULL,
    binary_sha256 TEXT NOT NULL,
    arguments TEXT[],
    env_variables JSONB,
    priority TEXT CHECK (priority IN ('foreground', 'background', 'best_effort')),
    status TEXT CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    executor_id TEXT,
    stdout TEXT,
    stderr TEXT,
    exit_code INTEGER,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    last_heartbeat TIMESTAMP
);

-- Job execution attempts history
CREATE TABLE job_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id) ON DELETE CASCADE,
    executor_id TEXT NOT NULL,
    executor_ip TEXT NOT NULL,
    started_at TIMESTAMP DEFAULT NOW(),
    ended_at TIMESTAMP,
    status TEXT CHECK (status IN ('running', 'completed', 'failed', 'timeout')),
    error_message TEXT
);

-- Indexes for performance
CREATE INDEX idx_jobs_completed_at ON jobs(completed_at) WHERE status IN ('completed', 'failed', 'cancelled');
CREATE INDEX idx_jobs_heartbeat ON jobs(last_heartbeat) WHERE status = 'running';
CREATE INDEX idx_jobs_pending ON jobs(priority, created_at) WHERE status = 'pending';
CREATE INDEX idx_job_attempts_job_id ON job_attempts(job_id);
```

### REST API Endpoints

All endpoints use JSON for request/response bodies. All timestamps in UTC format.

- `GET /api/v1/health` - Health check endpoint
  - Response: `{"status": "healthy", "database": "connected"}`
- `GET /api/v1/metrics` - Prometheus-compatible metrics
- `POST /api/v1/jobs` - Submit new job
- `GET /api/v1/jobs` - List jobs with filtering (by status, type, priority)
- `GET /api/v1/jobs/{id}` - Get job details including execution attempts history
- `DELETE /api/v1/jobs/{id}` - Cancel pending job only
- `POST /api/v1/jobs/claim` - Executor claims next available job (one at a time)
  - Request body: `{"executor_id": "worker1-abc123", "executor_ip": "192.168.1.100"}`
  - Response: Job details or 204 No Content if no jobs available
- `PUT /api/v1/jobs/{id}/heartbeat` - Update heartbeat for running job
- `PUT /api/v1/jobs/{id}/complete` - Mark job completed with stdout/stderr/exit code
- `PUT /api/v1/jobs/{id}/fail` - Mark job failed with error details

**Error Response Format:**
```json
{
  "error": "error message",
  "context": {
    "field": "additional context",
    "job_id": "uuid-if-relevant"
  }
}
```
Standard HTTP status codes: 200 OK, 201 Created, 204 No Content, 400 Bad Request, 404 Not Found, 500 Internal Server Error

### Implementation Notes

**Executor ID Generation:**
- Each executor generates a unique ID at startup: `{name}-{unique-suffix}`
- The name is provided via the `--name` flag (required)
- A unique suffix (e.g., UUID or timestamp-based) is appended automatically
- This allows multiple executors to run with the same logical name
- The executor ID is used for tracking job attempts, heartbeats, and logging
- Example: `worker1-a3f2c8d9` or `gpu-executor-1234567890`

**Binary Execution:**
- Downloaded binaries automatically get execute permissions (chmod +x)
- No size or timeout limits for binary downloads
- Binaries execute from cache location, not copied to work directory
- Cache keyed by SHA256 only (content-addressable storage)
- Architecture mismatches handled by OS (will fail naturally)

**Job Arguments:**
- Arguments passed as separate args to exec.Command(bin, args...)
- No escaping or quoting needed - OS handles it
- Arguments provided as array in job specification

**Job Type Field:**
- Free text string but cannot contain spaces (for filtering)
- Used in GET /api/v1/jobs filtering
- Examples: "data-processing", "ml-training", "report-generation"

**Job Concurrency:**
- Executors run 1 job by default, configurable via `--max-jobs`
- Claims happen one at a time even with concurrency enabled
- Each job gets its own goroutine and working directory
- Heartbeats managed per job independently

**Environment Variables:**
- Job environment variables completely replace executor environment
- No merging with existing environment variables
- Clean environment for each job execution

**Output Handling:**
- Maximum output size: 1MB for stdout and stderr each
- If output exceeds 1MB: keep first 500 lines, then as many lines from end as fit
- Output truncation happens before storing in database

**Network Resilience:**
- Executor stops claiming new jobs after 1 minute of network failures
- Existing jobs continue to run and retry sending results
- Server automatically reconnects to database on connection loss
- No in-memory queuing during database outages

**Graceful Shutdown:**
- Uses signal.NotifyContext for signal handling
- Context propagation throughout the application
- Running jobs allowed to complete before shutdown
- New job claims stopped immediately on shutdown signal

**Timestamps:**
- All timestamps stored and returned in UTC
- ISO 8601 format in JSON responses

**Submitter SHA256 Calculation:**
- Streams binary download for in-place SHA256 calculation
- No temporary file storage required
- Memory-efficient streaming approach
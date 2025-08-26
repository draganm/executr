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
   - Add required indexes for performance

2. **Write sqlc queries**
   - Job CRUD operations (create, get, list, update, delete)
   - Claim job with atomic executor assignment
   - Heartbeat update query
   - Find stale jobs query
   - Cancel job query (with status validation)
   - Cleanup old jobs query

3. **Generate database code**
   - Run sqlc generate
   - Create database connection helper

### Phase 3: Core Models and Shared Code
1. **Define core data structures**
   - Job model with all fields
   - JobResult model (stdout, stderr, exit code)
   - Priority enum (foreground, background, best_effort)
   - Status enum (pending, running, completed, failed, cancelled)

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
   - Connect to PostgreSQL
   - Run migrations on startup
   - Create connection pool

3. **Build REST API handlers**
   - POST /api/v1/jobs - Submit job
   - GET /api/v1/jobs - List jobs
   - GET /api/v1/jobs/{id} - Get job details
   - DELETE /api/v1/jobs/{id} - Cancel job
   - PUT /api/v1/jobs/{id}/claim - Claim job for execution
   - PUT /api/v1/jobs/{id}/heartbeat - Update heartbeat
   - PUT /api/v1/jobs/{id}/complete - Mark completed
   - PUT /api/v1/jobs/{id}/fail - Mark failed

4. **Add background workers**
   - Heartbeat monitor (check every 5 seconds)
   - Job cleaner (run every hour, delete old jobs)
   - Graceful shutdown handling

### Phase 5: Client Package
1. **Create client library structure**
   - Define client interface
   - Implement HTTP client with base URL

2. **Implement client methods**
   - SubmitJob(job) (jobID, error)
   - GetJob(jobID) (job, error)
   - ListJobs(filter) ([]job, error)
   - CancelJob(jobID) error
   - ClaimJob(executorID, priority) (job, error)
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
   - Generate executor ID if not provided

2. **Implement binary cache**
   - Create cache directory structure
   - SHA256 verification logic
   - Cache lookup by URL and SHA256
   - LRU eviction when size limit reached

3. **Build job execution engine**
   - Poll server for jobs by priority
   - Create temporary working directory for each job
   - Download and cache binaries
   - Execute binary with arguments and env vars in job directory
   - Capture stdout, stderr, exit code
   - Handle execution errors
   - Clean up job directory after completion

4. **Add heartbeat mechanism**
   - Start heartbeat goroutine when job claimed
   - Send heartbeat every 5 seconds
   - Stop on job completion/failure

5. **Implement job cleanup**
   - Create isolated working directory for each job under configurable root
   - Job directory path: {work-dir}/{job-id}/
   - Set job directory as current directory for binary execution
   - Clean up directory after job completion (success or failure)
   - Clean up orphaned directories on executor restart
   - Handle cleanup errors gracefully

### Phase 7: Submitter Implementation
1. **Create submit command**
   - Add submit subcommand to CLI
   - Parse job parameters from flags/env
   - Download binary to calculate SHA256 if not provided
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
   - Multiple executor coordination
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
- `--executor-id` / `EXECUTR_EXECUTOR_ID` - Unique executor ID (default: hostname-pid)
- `--cache-dir` / `EXECUTR_CACHE_DIR` - Binary cache directory (default: ~/.executr/cache)
- `--work-dir` / `EXECUTR_WORK_DIR` - Root directory for job working directories (default: /tmp/executr-jobs)
- `--poll-interval` / `EXECUTR_POLL_INTERVAL` - Job polling frequency (default: 5s)
- `--max-cache-size` / `EXECUTR_MAX_CACHE_SIZE` - Maximum cache size in MB (default: 1000)
- `--heartbeat-interval` / `EXECUTR_HEARTBEAT_INTERVAL` - Heartbeat frequency (default: 5s)
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

-- Indexes for performance
CREATE INDEX idx_jobs_completed_at ON jobs(completed_at) WHERE status IN ('completed', 'failed', 'cancelled');
CREATE INDEX idx_jobs_heartbeat ON jobs(last_heartbeat) WHERE status = 'running';
CREATE INDEX idx_jobs_pending ON jobs(priority, created_at) WHERE status = 'pending';
```

### REST API Endpoints

All endpoints use JSON for request/response bodies.

- `POST /api/v1/jobs` - Submit new job
- `GET /api/v1/jobs` - List jobs with filtering
- `GET /api/v1/jobs/{id}` - Get job details
- `DELETE /api/v1/jobs/{id}` - Cancel pending job
- `PUT /api/v1/jobs/{id}/claim` - Executor claims job
- `PUT /api/v1/jobs/{id}/heartbeat` - Update heartbeat
- `PUT /api/v1/jobs/{id}/complete` - Mark job completed with results
- `PUT /api/v1/jobs/{id}/fail` - Mark job failed with error details
# Executr

A distributed job execution system with binary caching, priority-based scheduling, and automatic retry capabilities.

## Features

- **Distributed Execution**: Multiple executors can claim and process jobs concurrently
- **Binary Caching**: Content-addressable binary cache with SHA256 verification
- **Priority Scheduling**: Three-tier priority system (foreground, background, best_effort)
- **Automatic Recovery**: Heartbeat monitoring and automatic job recovery on executor failure
- **Isolated Execution**: Each job runs in its own working directory
- **Output Capture**: Automatic capture and storage of stdout/stderr with smart truncation
- **Prometheus Metrics**: Built-in monitoring and metrics export
- **REST API**: Full-featured REST API for job management
- **CLI Tools**: Command-line interface for job submission and monitoring

## Quick Start

### Using Docker Compose

```bash
# Start PostgreSQL and Executr server
docker-compose up -d

# Submit a job
./executr submit \
  --binary-url https://example.com/binary \
  --type data-processing \
  --priority background \
  --args "input.txt" \
  --args "output.txt"

# Check job status
./executr status <job-id>

# Start an executor
./executr executor \
  --name worker-1 \
  --server-url http://localhost:8080 \
  --max-jobs 2
```

### Building from Source

```bash
# With Nix flake
direnv allow  # or: nix develop
go build ./cmd/executr

# Without Nix
go build ./cmd/executr
```

## Architecture

Executr consists of three main components:

1. **Server**: Central coordinator that manages jobs and assigns them to executors
2. **Executor**: Worker process that claims jobs, downloads binaries, and executes them
3. **CLI**: Command-line interface for submitting jobs and checking status

```
┌─────────────┐     REST API      ┌──────────────┐
│   CLI/SDK   ├──────────────────►│              │
└─────────────┘                    │    Server    │
                                   │              │
┌─────────────┐     Claim Jobs    │  PostgreSQL  │
│  Executor   │◄──────────────────┤              │
└─────────────┘     Heartbeats    └──────────────┘
```

## Components

### Server

The server provides REST API endpoints for job management and coordinates work distribution:

- Manages job queue with priority-based scheduling
- Monitors executor heartbeats and recovers stale jobs
- Automatically cleans up old completed jobs
- Provides Prometheus metrics for monitoring

### Executor

Executors are worker processes that execute jobs:

- Poll server for available work based on priority
- Maintain a local binary cache with SHA256 verification
- Execute jobs in isolated working directories
- Send heartbeats while processing jobs
- Support graceful shutdown with job completion

### CLI

The command-line interface provides tools for:

- Submitting jobs with automatic SHA256 calculation
- Querying job status and results
- Cancelling pending jobs
- Running server and executor processes

## Configuration

All components support both environment variables and command-line flags. See [Configuration Reference](docs/configuration.md) for details.

## API Documentation

See [API Documentation](docs/api.md) for complete REST API reference.

## Examples

See the [examples/](examples/) directory for:

- Job submission examples
- Executor configurations
- Docker Compose setup
- Kubernetes deployment

## Monitoring

Executr exposes Prometheus metrics at `/api/v1/metrics`:

- Job submission/completion rates
- Executor health and utilization
- Queue depths by priority
- Binary cache hit rates
- API request latencies

## Development

### Prerequisites

- Go 1.23+
- PostgreSQL 16+
- Docker (for testing)

### Running Tests

```bash
# Unit tests
go test ./...

# E2E tests
go test ./e2e -v

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Database Migrations

Migrations run automatically on server startup. To run manually:

```bash
migrate -path internal/server/migrations \
        -database "postgres://user:pass@localhost/executr" \
        up
```

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) - see the [LICENSE](LICENSE) file for details.

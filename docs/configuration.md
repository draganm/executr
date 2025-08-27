# Configuration Reference

All Executr components can be configured using either command-line flags or environment variables. Environment variables take precedence over command-line flags.

## Server Configuration

### Database Connection

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--db-url` | `EXECUTR_DB_URL` | Required | PostgreSQL connection string |

Example:
```bash
executr server --db-url "postgres://user:pass@localhost/executr?sslmode=require"
# or
EXECUTR_DB_URL="postgres://user:pass@localhost/executr?sslmode=require" executr server
```

### Network Settings

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--port` | `EXECUTR_PORT` | `8080` | HTTP server port |
| `--host` | `EXECUTR_HOST` | `0.0.0.0` | Bind address |

### Job Management

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--cleanup-interval` | `EXECUTR_CLEANUP_INTERVAL` | `1h` | How often to clean old jobs |
| `--job-retention` | `EXECUTR_JOB_RETENTION` | `48h` | Keep completed jobs for this duration |
| `--heartbeat-timeout` | `EXECUTR_HEARTBEAT_TIMEOUT` | `15s` | Mark job as stale after this timeout |
| `--max-output-size` | `EXECUTR_MAX_OUTPUT_SIZE` | `1048576` | Max bytes for stdout/stderr (1MB) |

### Logging

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--log-level` | `EXECUTR_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `--log-format` | `EXECUTR_LOG_FORMAT` | `json` | Log format (json/text) |

### Complete Server Example

```bash
executr server \
  --db-url "postgres://executr:password@db.example.com/executr" \
  --port 8080 \
  --cleanup-interval 30m \
  --job-retention 24h \
  --heartbeat-timeout 30s \
  --log-level info
```

## Executor Configuration

### Server Connection

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--server-url` | `EXECUTR_SERVER_URL` | Required | Server API endpoint |
| `--name` | `EXECUTR_NAME` | Required | Executor name (used as ID prefix) |

### Execution Settings

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--max-jobs` | `EXECUTR_MAX_JOBS` | `1` | Maximum concurrent jobs |
| `--poll-interval` | `EXECUTR_POLL_INTERVAL` | `5s` | How often to check for new jobs |
| `--heartbeat-interval` | `EXECUTR_HEARTBEAT_INTERVAL` | `5s` | How often to send heartbeats |
| `--network-timeout` | `EXECUTR_NETWORK_TIMEOUT` | `60s` | Stop claiming jobs after network failure |

### Storage Settings

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--cache-dir` | `EXECUTR_CACHE_DIR` | `~/.executr/cache` | Binary cache directory |
| `--work-dir` | `EXECUTR_WORK_DIR` | `/tmp/executr-jobs` | Job working directories |
| `--max-cache-size` | `EXECUTR_MAX_CACHE_SIZE` | `400` | Maximum cache size in MB |

### Logging

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--log-level` | `EXECUTR_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `--log-format` | `EXECUTR_LOG_FORMAT` | `json` | Log format (json/text) |

### Complete Executor Example

```bash
executr executor \
  --server-url http://executr.example.com:8080 \
  --name worker-gpu-1 \
  --max-jobs 4 \
  --poll-interval 3s \
  --cache-dir /mnt/ssd/cache \
  --work-dir /mnt/ssd/work \
  --max-cache-size 10000 \
  --log-level debug
```

## CLI Configuration (Submit/Status/Cancel)

### Server Connection

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--server-url` | `EXECUTR_SERVER_URL` | Required | Server API endpoint |

### Submit Command

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--binary-url` | `EXECUTR_BINARY_URL` | Required | URL to executable binary |
| `--binary-sha256` | `EXECUTR_BINARY_SHA256` | Auto-calculated | SHA256 hash of binary |
| `--type` | `EXECUTR_TYPE` | Required | Job type (no spaces) |
| `--priority` | `EXECUTR_PRIORITY` | `background` | Priority level |
| `--args` | - | - | Arguments (can be repeated) |
| `--env` | - | - | Environment vars KEY=VALUE (can be repeated) |
| `--output` | `EXECUTR_OUTPUT` | `table` | Output format (json/table) |

Example:
```bash
executr submit \
  --server-url http://localhost:8080 \
  --binary-url https://example.com/processor \
  --type data-processor \
  --priority foreground \
  --args "input.csv" \
  --args "output.json" \
  --env "DEBUG=true" \
  --env "WORKERS=4" \
  --output json
```

### Status Command

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--output` | `EXECUTR_OUTPUT` | `table` | Output format (json/table) |

Example:
```bash
executr status <job-id> \
  --server-url http://localhost:8080 \
  --output json
```

### Cancel Command

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--output` | `EXECUTR_OUTPUT` | `table` | Output format (json/table) |

Example:
```bash
executr cancel <job-id> \
  --server-url http://localhost:8080
```

## Environment Variable Files

You can use `.env` files with tools like `direnv` or `systemd` EnvironmentFile:

### .env.server
```bash
EXECUTR_DB_URL=postgres://executr:password@localhost/executr
EXECUTR_PORT=8080
EXECUTR_LOG_LEVEL=info
EXECUTR_CLEANUP_INTERVAL=1h
EXECUTR_JOB_RETENTION=48h
```

### .env.executor
```bash
EXECUTR_SERVER_URL=http://localhost:8080
EXECUTR_NAME=worker-1
EXECUTR_MAX_JOBS=2
EXECUTR_CACHE_DIR=/var/cache/executr
EXECUTR_WORK_DIR=/var/lib/executr/work
EXECUTR_LOG_LEVEL=info
```

### .env.client
```bash
EXECUTR_SERVER_URL=http://localhost:8080
EXECUTR_OUTPUT=json
```

## Duration Formats

Duration values support the following formats:
- `300ms` - 300 milliseconds
- `5s` - 5 seconds
- `10m` - 10 minutes
- `2h` - 2 hours
- `24h` - 24 hours

## Size Formats

Size values are specified in megabytes:
- `400` - 400 MB
- `1000` - 1 GB
- `10000` - 10 GB

## Connection String Format

PostgreSQL connection strings follow the standard format:

```
postgres://[user[:password]@][host][:port][/dbname][?param1=value1&...]
```

Common parameters:
- `sslmode`: `disable`, `require`, `verify-ca`, `verify-full`
- `pool_max_conns`: Maximum connections in pool (default: 25)
- `pool_min_conns`: Minimum connections in pool (default: 0)
- `pool_max_conn_lifetime`: Maximum connection lifetime
- `pool_max_conn_idle_time`: Maximum idle time

Example with pool settings:
```
postgres://user:pass@localhost/executr?sslmode=require&pool_max_conns=50
```

## Priority Levels

Jobs can have one of three priority levels:

1. **foreground**: Highest priority, for interactive or time-sensitive jobs
2. **background**: Normal priority, for regular processing
3. **best_effort**: Lowest priority, for non-critical work

Jobs are executed in priority order, with older jobs of the same priority executed first.

## Log Levels

Available log levels from most to least verbose:

1. **debug**: Detailed debugging information
2. **info**: Informational messages
3. **warn**: Warning messages
4. **error**: Error messages only

## Security Considerations

1. **Database Credentials**: Use environment variables or secrets management, not command-line flags
2. **Binary URLs**: Always use HTTPS for binary URLs in production
3. **SHA256 Verification**: Always provide SHA256 hashes for security
4. **File Permissions**: Ensure proper permissions on cache and work directories
5. **Network Security**: Use TLS for database connections in production

## Performance Tuning

### Server
- Increase `pool_max_conns` for high executor counts
- Adjust `heartbeat-timeout` based on job characteristics
- Set appropriate `job-retention` to manage database size

### Executor
- Increase `max-jobs` for CPU-bound workloads
- Decrease `poll-interval` for lower latency
- Increase `max-cache-size` for binary-heavy workloads
- Use SSD storage for cache and work directories

### Database
- Add appropriate indexes (included in migrations)
- Regular VACUUM and ANALYZE
- Monitor connection pool usage
- Consider read replicas for high query loads
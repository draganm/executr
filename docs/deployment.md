# Deployment Guide

## Prerequisites

- PostgreSQL 16+ database
- Go 1.23+ (for building from source)
- Docker (optional, for containerized deployment)

## Deployment Options

### 1. Docker Compose (Recommended for Development)

The easiest way to get started:

```bash
# Clone the repository
git clone https://github.com/draganm/executr.git
cd executr

# Start services
docker-compose up -d

# Check logs
docker-compose logs -f

# Stop services
docker-compose down
```

### 2. Systemd Services (Production)

#### Install Binary

```bash
# Build from source
go build -o /usr/local/bin/executr ./cmd/executr

# Or download pre-built binary (when available)
wget https://github.com/draganm/executr/releases/latest/download/executr-linux-amd64
chmod +x executr-linux-amd64
sudo mv executr-linux-amd64 /usr/local/bin/executr
```

#### PostgreSQL Setup

```sql
-- Create database and user
CREATE DATABASE executr;
CREATE USER executr WITH PASSWORD 'secure-password';
GRANT ALL PRIVILEGES ON DATABASE executr TO executr;
```

#### Server Service

Create `/etc/systemd/system/executr-server.service`:

```ini
[Unit]
Description=Executr Server
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=executr
Group=executr
WorkingDirectory=/var/lib/executr
ExecStart=/usr/local/bin/executr server
Restart=always
RestartSec=10

# Environment
Environment="EXECUTR_DB_URL=postgres://executr:password@localhost/executr?sslmode=require"
Environment="EXECUTR_PORT=8080"
Environment="EXECUTR_LOG_LEVEL=info"
Environment="EXECUTR_CLEANUP_INTERVAL=1h"
Environment="EXECUTR_JOB_RETENTION=48h"

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/executr

[Install]
WantedBy=multi-user.target
```

#### Executor Service

Create `/etc/systemd/system/executr-executor@.service`:

```ini
[Unit]
Description=Executr Executor %i
After=network.target executr-server.service
Wants=executr-server.service

[Service]
Type=simple
User=executr
Group=executr
WorkingDirectory=/var/lib/executr
ExecStart=/usr/local/bin/executr executor
Restart=always
RestartSec=10

# Environment
Environment="EXECUTR_SERVER_URL=http://localhost:8080"
Environment="EXECUTR_NAME=worker-%i"
Environment="EXECUTR_CACHE_DIR=/var/cache/executr-%i"
Environment="EXECUTR_WORK_DIR=/var/lib/executr/work-%i"
Environment="EXECUTR_MAX_JOBS=2"
Environment="EXECUTR_LOG_LEVEL=info"

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/executr /var/cache/executr-%i

[Install]
WantedBy=multi-user.target
```

#### Start Services

```bash
# Create user and directories
sudo useradd -r -s /bin/false executr
sudo mkdir -p /var/lib/executr /var/cache/executr-{1,2,3}
sudo chown -R executr:executr /var/lib/executr /var/cache/executr-*

# Enable and start services
sudo systemctl daemon-reload
sudo systemctl enable executr-server
sudo systemctl start executr-server

# Start multiple executors
sudo systemctl enable executr-executor@{1,2,3}
sudo systemctl start executr-executor@{1,2,3}

# Check status
sudo systemctl status executr-server
sudo systemctl status executr-executor@1
```

### 3. Kubernetes Deployment

#### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: executr-config
data:
  server.env: |
    EXECUTR_PORT=8080
    EXECUTR_LOG_LEVEL=info
    EXECUTR_CLEANUP_INTERVAL=1h
    EXECUTR_JOB_RETENTION=48h
    EXECUTR_HEARTBEAT_TIMEOUT=15s
  
  executor.env: |
    EXECUTR_SERVER_URL=http://executr-server:8080
    EXECUTR_MAX_JOBS=2
    EXECUTR_POLL_INTERVAL=5s
    EXECUTR_MAX_CACHE_SIZE=400
    EXECUTR_LOG_LEVEL=info
```

#### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: executr-db
type: Opaque
stringData:
  database-url: "postgres://executr:password@postgres:5432/executr?sslmode=require"
```

#### Server Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: executr-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: executr-server
  template:
    metadata:
      labels:
        app: executr-server
    spec:
      containers:
      - name: server
        image: executr:latest
        command: ["executr", "server"]
        ports:
        - containerPort: 8080
        env:
        - name: EXECUTR_DB_URL
          valueFrom:
            secretKeyRef:
              name: executr-db
              key: database-url
        envFrom:
        - configMapRef:
            name: executr-config
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: executr-server
spec:
  selector:
    app: executr-server
  ports:
  - port: 8080
    targetPort: 8080
```

#### Executor StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: executr-executor
spec:
  serviceName: executr-executor
  replicas: 3
  selector:
    matchLabels:
      app: executr-executor
  template:
    metadata:
      labels:
        app: executr-executor
    spec:
      containers:
      - name: executor
        image: executr:latest
        command: ["executr", "executor"]
        env:
        - name: EXECUTR_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: EXECUTR_CACHE_DIR
          value: /cache
        - name: EXECUTR_WORK_DIR
          value: /work
        envFrom:
        - configMapRef:
            name: executr-config
        volumeMounts:
        - name: cache
          mountPath: /cache
        - name: work
          mountPath: /work
  volumeClaimTemplates:
  - metadata:
      name: cache
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
  - metadata:
      name: work
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 20Gi
```

## Production Considerations

### Database

1. **Connection Pooling**: The default pgx pool settings are used. Adjust if needed:
   ```
   EXECUTR_DB_URL="postgres://user:pass@host/db?pool_max_conns=25"
   ```

2. **Backups**: Regular PostgreSQL backups are essential:
   ```bash
   pg_dump executr > backup.sql
   ```

3. **High Availability**: Consider PostgreSQL replication for HA.

### Security

1. **Network Security**:
   - Use TLS for PostgreSQL connections
   - Run server behind a reverse proxy (nginx, Traefik)
   - Implement authentication/authorization

2. **Binary Verification**:
   - Always provide SHA256 hashes for binaries
   - Consider signing binaries for additional security

3. **Resource Limits**:
   - Set memory/CPU limits for executors
   - Configure max cache size appropriately
   - Monitor disk usage for work directories

### Monitoring

1. **Prometheus Metrics**:
   ```yaml
   - job_name: 'executr'
     static_configs:
     - targets: ['executr-server:8080']
   ```

2. **Key Metrics to Monitor**:
   - `executr_jobs_submitted_total`
   - `executr_jobs_completed_total`
   - `executr_jobs_failed_total`
   - `executr_queue_depth`
   - `executr_executors_active`
   - `executr_cache_hit_ratio`

3. **Alerting Rules**:
   ```yaml
   - alert: HighJobFailureRate
     expr: rate(executr_jobs_failed_total[5m]) > 0.1
     annotations:
       summary: "High job failure rate"
   
   - alert: NoActiveExecutors
     expr: executr_executors_active == 0
     annotations:
       summary: "No active executors"
   
   - alert: StaleJobs
     expr: executr_jobs_stale > 0
     for: 5m
     annotations:
       summary: "Jobs stuck in running state"
   ```

### Scaling

1. **Horizontal Scaling**:
   - Add more executors for increased throughput
   - Single server can handle thousands of jobs
   - Use connection pooler (PgBouncer) for many executors

2. **Vertical Scaling**:
   - Increase executor `MAX_JOBS` for CPU-bound work
   - Increase cache size for binary-heavy workloads
   - Adjust PostgreSQL resources as needed

### Backup and Recovery

1. **Database Backup**:
   ```bash
   # Backup
   pg_dump -Fc executr > executr_backup.dump
   
   # Restore
   pg_restore -d executr executr_backup.dump
   ```

2. **Binary Cache**:
   - Cache can be rebuilt from URLs
   - Consider backing up for offline operation

### Troubleshooting

1. **Check Logs**:
   ```bash
   # Systemd
   journalctl -u executr-server -f
   journalctl -u executr-executor@1 -f
   
   # Docker
   docker-compose logs -f server
   docker-compose logs -f executor-1
   ```

2. **Database Connection Issues**:
   ```bash
   # Test connection
   psql $EXECUTR_DB_URL -c "SELECT 1"
   
   # Check migrations
   psql $EXECUTR_DB_URL -c "SELECT * FROM schema_migrations"
   ```

3. **Executor Issues**:
   ```bash
   # Check executor registration
   curl http://localhost:8080/api/v1/admin/executors
   
   # Check job queue
   curl http://localhost:8080/api/v1/admin/stats
   ```

## Health Checks

### Server Health

```bash
curl http://localhost:8080/api/v1/health
```

Expected response:
```json
{
  "status": "healthy",
  "database": "connected"
}
```

### Executor Health

Check executor logs for:
- Successful job claims
- Heartbeat confirmations
- Binary cache hits

## Maintenance

### Database Maintenance

```sql
-- Vacuum and analyze for performance
VACUUM ANALYZE jobs;
VACUUM ANALYZE job_attempts;

-- Check table sizes
SELECT 
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables 
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

### Cache Cleanup

The executor automatically manages cache with LRU eviction. Manual cleanup:

```bash
# Remove all cached binaries
rm -rf /var/cache/executr-*/

# Remove old work directories
find /var/lib/executr/work-* -type d -mtime +7 -exec rm -rf {} \;
```

## Upgrading

1. **Database Migrations**:
   - Migrations run automatically on server start
   - Always backup before upgrading

2. **Rolling Updates**:
   ```bash
   # Update executors first (they're stateless)
   for i in {1..3}; do
     sudo systemctl stop executr-executor@$i
     # Update binary
     sudo systemctl start executr-executor@$i
     sleep 30
   done
   
   # Then update server
   sudo systemctl stop executr-server
   # Update binary
   sudo systemctl start executr-server
   ```

3. **Version Compatibility**:
   - Executors are forward-compatible with server
   - Always upgrade executors before server
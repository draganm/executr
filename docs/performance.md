# Performance Tuning Guide

## Database Performance

### Connection Pool Settings

Configure the PostgreSQL connection pool for your workload:

```bash
# Default settings (good for most cases)
EXECUTR_DB_URL="postgres://user:pass@host/db?pool_max_conns=25&pool_min_conns=5"

# High concurrency (many executors)
EXECUTR_DB_URL="postgres://user:pass@host/db?pool_max_conns=100&pool_min_conns=20"

# Low resource environment
EXECUTR_DB_URL="postgres://user:pass@host/db?pool_max_conns=10&pool_min_conns=2"
```

### PostgreSQL Tuning

Key PostgreSQL settings for Executr:

```sql
-- postgresql.conf adjustments

-- Increase for high write throughput
shared_buffers = 256MB          # 25% of RAM for dedicated server
effective_cache_size = 1GB      # 50-75% of RAM
work_mem = 4MB                  # Per sort/hash operation
maintenance_work_mem = 64MB     # For VACUUM, indexes

-- Connection settings
max_connections = 200            # Adjust based on executor count
max_prepared_transactions = 100 # For prepared statements

-- Write performance
checkpoint_segments = 16        # Reduce checkpoint frequency
checkpoint_completion_target = 0.9
wal_buffers = 16MB

-- Query planning
random_page_cost = 1.1          # For SSD storage
effective_io_concurrency = 200  # For SSD storage
```

### Index Maintenance

Regular maintenance tasks:

```sql
-- Run during low-activity periods
VACUUM ANALYZE jobs;
VACUUM ANALYZE job_attempts;

-- Rebuild indexes if fragmented
REINDEX TABLE jobs;
REINDEX TABLE job_attempts;

-- Check index usage
SELECT 
  schemaname,
  tablename,
  indexname,
  idx_scan,
  idx_tup_read,
  idx_tup_fetch
FROM pg_stat_user_indexes
WHERE schemaname = 'public'
ORDER BY idx_scan DESC;
```

## Executor Performance

### Concurrency Settings

Tune based on job characteristics:

```bash
# CPU-bound jobs (computation heavy)
EXECUTR_MAX_JOBS=<number_of_cpu_cores>

# I/O-bound jobs (network/disk heavy)
EXECUTR_MAX_JOBS=$((number_of_cpu_cores * 2))

# Memory-constrained environment
EXECUTR_MAX_JOBS=<available_memory_gb / job_memory_gb>
```

### Cache Optimization

Binary cache tuning:

```bash
# Large binaries, frequent reuse
EXECUTR_MAX_CACHE_SIZE=10000  # 10GB

# Small binaries, high variety
EXECUTR_MAX_CACHE_SIZE=1000   # 1GB

# Limited disk space
EXECUTR_MAX_CACHE_SIZE=100    # 100MB
```

Cache placement:

- Use SSD for cache directory
- Separate cache from work directory
- Consider tmpfs for small, frequently-used binaries

### Polling Optimization

Balance between latency and load:

```bash
# Low latency requirement
EXECUTR_POLL_INTERVAL=1s

# Normal operation
EXECUTR_POLL_INTERVAL=5s

# Reduce database load
EXECUTR_POLL_INTERVAL=10s
```

## Server Performance

### API Optimization

1. **Enable HTTP/2**:
   ```nginx
   # nginx reverse proxy
   server {
     listen 443 ssl http2;
     # ... rest of config
   }
   ```

2. **Response Compression**:
   ```nginx
   gzip on;
   gzip_types application/json;
   gzip_comp_level 6;
   ```

3. **Connection Limits**:
   ```nginx
   limit_conn_zone $binary_remote_addr zone=addr:10m;
   limit_conn addr 100;
   ```

### Background Workers

Tune cleanup intervals:

```bash
# Aggressive cleanup (high job volume)
EXECUTR_CLEANUP_INTERVAL=15m
EXECUTR_JOB_RETENTION=12h

# Conservative cleanup (audit requirements)
EXECUTR_CLEANUP_INTERVAL=6h
EXECUTR_JOB_RETENTION=168h  # 7 days

# Balanced approach
EXECUTR_CLEANUP_INTERVAL=1h
EXECUTR_JOB_RETENTION=48h
```

## Monitoring Key Metrics

### Database Metrics

Monitor these PostgreSQL metrics:

```sql
-- Connection pool usage
SELECT count(*) FROM pg_stat_activity;

-- Long-running queries
SELECT 
  pid, 
  now() - query_start AS duration, 
  query 
FROM pg_stat_activity
WHERE state = 'active'
  AND now() - query_start > '1 second'::interval
ORDER BY duration DESC;

-- Table sizes
SELECT 
  relname AS table,
  pg_size_pretty(pg_total_relation_size(relid)) AS size
FROM pg_stat_user_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(relid) DESC;

-- Index hit ratio (should be > 0.99)
SELECT 
  sum(idx_blks_hit) / NULLIF(sum(idx_blks_hit + idx_blks_read), 0) AS index_hit_ratio
FROM pg_statio_user_indexes;
```

### Application Metrics

Key Prometheus metrics to watch:

```yaml
# High-level health
- executr_jobs_submitted_total
- executr_jobs_completed_total
- executr_jobs_failed_total
- executr_queue_depth

# Performance indicators
- executr_job_duration_seconds
- executr_api_request_duration_seconds
- executr_cache_hit_ratio
- executr_executor_utilization

# Capacity planning
- executr_executors_active
- executr_jobs_waiting_time_seconds
- executr_database_connections_active
```

## Bottleneck Analysis

### Database Bottlenecks

Signs and solutions:

| Symptom | Likely Cause | Solution |
|---------|--------------|----------|
| High connection count | Pool exhaustion | Increase pool_max_conns |
| Slow job claims | Missing indexes | Check query plans, add indexes |
| Lock contention | Concurrent updates | Use advisory locks |
| High CPU on DB | Inefficient queries | Optimize queries, add indexes |
| High I/O wait | Slow disks | Move to SSD, increase RAM |

### Executor Bottlenecks

| Symptom | Likely Cause | Solution |
|---------|--------------|----------|
| Low job throughput | Not enough executors | Scale horizontally |
| High memory usage | Large outputs | Reduce MAX_JOBS |
| Slow binary downloads | Network bandwidth | Local mirror, increase timeout |
| Cache misses | Cache too small | Increase MAX_CACHE_SIZE |
| Disk full | Work dir not cleaned | Check cleanup, increase disk |

### Server Bottlenecks

| Symptom | Likely Cause | Solution |
|---------|--------------|----------|
| High API latency | Database queries | Add indexes, optimize queries |
| Memory growth | Output accumulation | Reduce retention period |
| CPU spikes | Metrics calculation | Reduce scrape frequency |
| Slow job listing | Large result sets | Add pagination, filters |

## Scaling Strategies

### Vertical Scaling

When to scale up:

- Database CPU consistently > 80%
- Memory pressure on executors
- Network bandwidth saturation
- Disk I/O bottlenecks

### Horizontal Scaling

When to scale out:

- Queue depth growing
- Job wait times increasing
- Executor utilization > 90%
- Geographic distribution needed

### Sharding Strategies

For very large deployments:

1. **By Job Type**: Different servers for different job types
2. **By Priority**: Dedicated infrastructure for foreground jobs
3. **By Region**: Regional servers for lower latency
4. **By Customer**: Multi-tenant isolation

## Best Practices

### Job Design

1. **Minimize Output**: Keep stdout/stderr concise
2. **Batch Operations**: Combine small jobs when possible
3. **Appropriate Priority**: Use best_effort for non-critical work
4. **Efficient Binaries**: Optimize binary size and startup time

### Executor Deployment

1. **Dedicated Hardware**: Avoid noisy neighbors
2. **Local Storage**: Use local SSD for cache/work
3. **Network Proximity**: Deploy close to server
4. **Resource Monitoring**: Track CPU, memory, disk usage

### Database Management

1. **Regular Maintenance**: Schedule VACUUM and ANALYZE
2. **Monitor Growth**: Track table and index sizes
3. **Backup Strategy**: Regular backups, test restores
4. **Version Updates**: Keep PostgreSQL updated

## Benchmarking

Sample benchmarking script:

```bash
#!/bin/bash
# Benchmark job throughput

SERVER_URL="http://localhost:8080"
NUM_JOBS=1000
CONCURRENCY=10

# Submit jobs
echo "Submitting $NUM_JOBS jobs..."
START=$(date +%s)

for i in $(seq 1 $NUM_JOBS); do
  (
    executr submit \
      --server-url $SERVER_URL \
      --binary-url "https://example.com/test-binary" \
      --type "benchmark-$((i % CONCURRENCY))" \
      --priority background \
      --output json > /dev/null
  ) &
  
  if [ $((i % CONCURRENCY)) -eq 0 ]; then
    wait
  fi
done
wait

END=$(date +%s)
DURATION=$((END - START))

echo "Submitted $NUM_JOBS jobs in $DURATION seconds"
echo "Rate: $((NUM_JOBS / DURATION)) jobs/second"

# Wait for completion
echo "Waiting for jobs to complete..."
while true; do
  PENDING=$(curl -s "$SERVER_URL/api/v1/admin/stats" | jq '.jobs_by_status.pending // 0')
  RUNNING=$(curl -s "$SERVER_URL/api/v1/admin/stats" | jq '.jobs_by_status.running // 0')
  
  if [ "$PENDING" -eq 0 ] && [ "$RUNNING" -eq 0 ]; then
    break
  fi
  
  echo "Pending: $PENDING, Running: $RUNNING"
  sleep 5
done

COMPLETE_END=$(date +%s)
TOTAL_DURATION=$((COMPLETE_END - START))

echo "All jobs completed in $TOTAL_DURATION seconds"
echo "Throughput: $((NUM_JOBS / TOTAL_DURATION)) jobs/second"
```

## Troubleshooting Performance Issues

### Slow Job Claims

```sql
-- Check claim query performance
EXPLAIN ANALYZE
UPDATE jobs
SET status = 'running',
    executor_id = 'test',
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
        created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING *;
```

### High Memory Usage

```bash
# Check memory consumers
ps aux | sort -k6 -rn | head -10

# Check cache directory size
du -sh /var/cache/executr/*

# Check work directory for orphaned jobs
find /var/lib/executr/work -type d -mtime +1
```

### Network Issues

```bash
# Test connectivity
curl -w "@curl-format.txt" -o /dev/null -s http://server:8080/api/v1/health

# Check DNS resolution
dig server
nslookup server

# Monitor network traffic
iftop -i eth0
nethogs
```
# Quick Start Guide

## Prerequisites

- Docker and Docker Compose installed
- Or: Go 1.23+ and PostgreSQL 16+ for local development

## 5-Minute Setup with Docker

1. **Clone the repository:**
   ```bash
   git clone https://github.com/draganm/executr.git
   cd executr
   ```

2. **Start all services:**
   ```bash
   docker-compose up -d
   ```

3. **Submit your first job:**
   ```bash
   # Build the CLI tool
   go build -o executr ./cmd/executr
   
   # Submit a test job
   ./executr submit \
     --server-url http://localhost:8080 \
     --binary-url https://github.com/your-org/tool/releases/download/v1.0/binary \
     --type test-job \
     --priority background
   ```

4. **Check job status:**
   ```bash
   # Get job ID from submit output, then:
   ./executr status <job-id> --server-url http://localhost:8080
   ```

5. **View metrics:**
   - Prometheus: http://localhost:9090
   - Grafana: http://localhost:3000 (admin/admin)

## Local Development Setup

1. **Start PostgreSQL:**
   ```bash
   # Using Docker
   docker run -d \
     --name executr-postgres \
     -e POSTGRES_DB=executr \
     -e POSTGRES_USER=executr \
     -e POSTGRES_PASSWORD=executr \
     -p 5432:5432 \
     postgres:16-alpine
   ```

2. **Build and start the server:**
   ```bash
   go build -o executr ./cmd/executr
   
   ./executr server \
     --db-url "postgres://executr:executr@localhost/executr?sslmode=disable" \
     --port 8080
   ```

3. **Start an executor:**
   ```bash
   ./executr executor \
     --server-url http://localhost:8080 \
     --name worker-1 \
     --max-jobs 2
   ```

4. **Submit and monitor jobs:**
   ```bash
   # Submit job
   JOB_ID=$(./executr submit \
     --server-url http://localhost:8080 \
     --binary-url https://example.com/binary \
     --type data-processing \
     --output json | jq -r .id)
   
   # Check status
   ./executr status $JOB_ID --server-url http://localhost:8080
   ```

## First Real Job

Here's an example of running a real data processing job:

```bash
# Create a simple script
cat > process.sh << 'EOF'
#!/bin/bash
echo "Processing started at $(date)"
echo "Arguments: $@"
echo "Environment: KEY1=$KEY1, KEY2=$KEY2"
sleep 5
echo "Processing completed at $(date)"
exit 0
EOF

# Upload to a web server (or use GitHub releases)
# For testing, you can use a local HTTP server:
python3 -m http.server 8000 &

# Submit the job
./executr submit \
  --server-url http://localhost:8080 \
  --binary-url http://localhost:8000/process.sh \
  --type batch-processing \
  --priority foreground \
  --args "input.csv" \
  --args "output.json" \
  --env "KEY1=value1" \
  --env "KEY2=value2"
```

## Common Use Cases

### Batch Processing
```bash
# Submit multiple jobs
for file in data/*.csv; do
  ./executr submit \
    --server-url http://localhost:8080 \
    --binary-url https://example.com/processor \
    --type csv-processing \
    --args "$file"
done
```

### CI/CD Pipeline
```bash
# Build job
BUILD_JOB=$(./executr submit \
  --binary-url https://example.com/builder \
  --type build \
  --priority foreground \
  --env "BRANCH=$GITHUB_REF" \
  --output json | jq -r .id)

# Wait for completion
while [[ $(./executr status $BUILD_JOB --output json | jq -r .status) == "pending" || \
         $(./executr status $BUILD_JOB --output json | jq -r .status) == "running" ]]; do
  sleep 5
done

# Check if successful
if [[ $(./executr status $BUILD_JOB --output json | jq -r .status) == "completed" ]]; then
  echo "Build successful!"
else
  echo "Build failed!"
  exit 1
fi
```

### ML Training
```bash
# Submit training job with GPU executor
./executr submit \
  --server-url http://localhost:8080 \
  --binary-url https://example.com/train-model \
  --type ml-training \
  --priority foreground \
  --env "MODEL=bert-base" \
  --env "EPOCHS=10" \
  --env "BATCH_SIZE=32" \
  --env "LEARNING_RATE=0.001"
```

## Monitoring

### Check System Status
```bash
# Overall statistics
curl http://localhost:8080/api/v1/admin/stats | jq

# Active executors
curl http://localhost:8080/api/v1/admin/executors | jq

# Pending jobs
curl "http://localhost:8080/api/v1/jobs?status=pending" | jq
```

### View Logs
```bash
# Docker Compose
docker-compose logs -f server
docker-compose logs -f executor-1

# Local development
# Logs are output to stdout/stderr
```

## Troubleshooting

### Job Stuck in Pending
```bash
# Check if executors are running
curl http://localhost:8080/api/v1/admin/executors

# Check executor logs
docker-compose logs executor-1

# Manually claim a job (for testing)
curl -X POST http://localhost:8080/api/v1/jobs/claim \
  -H "Content-Type: application/json" \
  -d '{"executor_id": "manual-test", "executor_ip": "127.0.0.1"}'
```

### Database Connection Issues
```bash
# Test database connection
docker exec -it executr-postgres psql -U executr -c "SELECT 1"

# Check server logs for migration errors
docker-compose logs server | grep -i migration
```

### Binary Download Failures
```bash
# Test binary URL is accessible
curl -I https://example.com/binary

# Check executor cache
docker exec -it executr_executor-1_1 ls -la /cache

# Calculate SHA256 manually
curl -s https://example.com/binary | sha256sum
```

## Next Steps

1. **Production Deployment**: See [Deployment Guide](deployment.md)
2. **API Integration**: See [API Documentation](api.md)
3. **Configuration**: See [Configuration Reference](configuration.md)
4. **Performance Tuning**: See [Performance Guide](performance.md)

## Getting Help

- GitHub Issues: https://github.com/draganm/executr/issues
- Documentation: https://github.com/draganm/executr/tree/main/docs

## Clean Up

To stop and remove all Docker containers:

```bash
docker-compose down -v
```

To remove built binaries:

```bash
rm ./executr
```
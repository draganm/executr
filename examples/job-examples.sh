#!/bin/bash
# Example job submission scripts

# Set server URL
export EXECUTR_SERVER_URL=${EXECUTR_SERVER_URL:-http://localhost:8080}

# Example 1: Simple job with arguments
executr submit \
  --binary-url https://github.com/user/tool/releases/download/v1.0/processor \
  --binary-sha256 abc123def456... \
  --type data-processing \
  --priority background \
  --args "input.csv" \
  --args "output.json" \
  --args "--verbose"

# Example 2: Job with environment variables
executr submit \
  --binary-url https://example.com/ml-trainer \
  --type ml-training \
  --priority foreground \
  --env "MODEL_TYPE=bert" \
  --env "BATCH_SIZE=32" \
  --env "EPOCHS=10" \
  --env "LEARNING_RATE=0.001"

# Example 3: Best effort batch job
executr submit \
  --binary-url https://example.com/report-generator \
  --type report-generation \
  --priority best_effort \
  --args "--format=pdf" \
  --args "--template=monthly" \
  --env "MONTH=2024-01"

# Example 4: Auto-calculate SHA256
executr submit \
  --binary-url https://example.com/analyzer \
  --type log-analysis \
  --priority background \
  --args "/logs/access.log"
  # SHA256 will be calculated automatically

# Example 5: Submit multiple jobs in a loop
for i in {1..10}; do
  executr submit \
    --binary-url https://example.com/worker \
    --type batch-job \
    --priority background \
    --args "chunk-$i.dat" \
    --output json | jq -r .id
done

# Example 6: Submit and wait for completion
JOB_ID=$(executr submit \
  --binary-url https://example.com/processor \
  --type quick-task \
  --priority foreground \
  --output json | jq -r .id)

echo "Submitted job: $JOB_ID"

# Poll for completion
while true; do
  STATUS=$(executr status $JOB_ID --output json | jq -r .status)
  echo "Status: $STATUS"
  
  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
    break
  fi
  
  sleep 5
done

# Get final results
executr status $JOB_ID

# Example 7: Bulk submission with error handling
submit_job() {
  local url=$1
  local type=$2
  
  executr submit \
    --binary-url "$url" \
    --type "$type" \
    --priority background \
    --output json 2>/dev/null
  
  if [ $? -eq 0 ]; then
    echo "Successfully submitted $type job"
  else
    echo "Failed to submit $type job" >&2
    return 1
  fi
}

# Submit different job types
submit_job "https://example.com/etl" "etl-job"
submit_job "https://example.com/backup" "backup-job"
submit_job "https://example.com/cleanup" "cleanup-job"

# Example 8: Cancel pending jobs of a specific type
for job_id in $(executr list --type batch-job --status pending --output json | jq -r '.[].id'); do
  executr cancel $job_id
  echo "Cancelled job: $job_id"
done
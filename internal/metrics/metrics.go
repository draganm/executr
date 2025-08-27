package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Job metrics
	JobsSubmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_jobs_submitted_total",
			Help: "Total number of jobs submitted",
		},
		[]string{"type", "priority"},
	)

	JobsCompleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_jobs_completed_total",
			Help: "Total number of jobs completed successfully",
		},
		[]string{"type", "priority", "executor"},
	)

	JobsFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_jobs_failed_total",
			Help: "Total number of jobs failed",
		},
		[]string{"type", "priority", "executor"},
	)

	JobsCancelled = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "executr_jobs_cancelled_total",
			Help: "Total number of jobs cancelled",
		},
	)

	JobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "executr_job_duration_seconds",
			Help:    "Job execution duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15), // 0.1s to ~1.6h
		},
		[]string{"type", "priority", "status"},
	)

	JobWaitTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "executr_job_wait_time_seconds",
			Help:    "Time jobs spend waiting to be executed",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		},
		[]string{"type", "priority"},
	)

	// Queue metrics
	JobsInQueue = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "executr_jobs_in_queue",
			Help: "Number of jobs currently in queue",
		},
		[]string{"status", "priority"},
	)

	// Executor metrics
	ExecutorsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "executr_executors_active",
			Help: "Number of active executors",
		},
	)

	ExecutorHeartbeats = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_executor_heartbeats_total",
			Help: "Total number of heartbeats received from executors",
		},
		[]string{"executor"},
	)

	ExecutorJobsClaimed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_executor_jobs_claimed_total",
			Help: "Total number of jobs claimed by executors",
		},
		[]string{"executor"},
	)

	// Binary cache metrics
	BinaryCacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_binary_cache_hits_total",
			Help: "Total number of binary cache hits",
		},
		[]string{"executor"},
	)

	BinaryCacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_binary_cache_misses_total",
			Help: "Total number of binary cache misses",
		},
		[]string{"executor"},
	)

	BinaryCacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "executr_binary_cache_size_bytes",
			Help: "Current size of binary cache in bytes",
		},
		[]string{"executor"},
	)

	BinaryDownloadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "executr_binary_download_duration_seconds",
			Help:    "Time taken to download binaries",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 12), // 0.1s to ~409s
		},
		[]string{"executor"},
	)

	// Database metrics
	DatabaseQueries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_database_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "status"},
	)

	DatabaseDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "executr_database_query_duration_seconds",
			Help:    "Database query duration",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		},
		[]string{"operation"},
	)

	DatabaseConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "executr_database_connections",
			Help: "Number of active database connections",
		},
	)

	// API metrics
	APIRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "executr_api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "executr_api_request_duration_seconds",
			Help:    "API request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// System metrics
	StaleJobsRecovered = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "executr_stale_jobs_recovered_total",
			Help: "Total number of stale jobs recovered",
		},
	)

	OldJobsCleaned = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "executr_old_jobs_cleaned_total",
			Help: "Total number of old jobs cleaned",
		},
	)
)

// Helper function to track executor status
func UpdateExecutorCount(delta float64) {
	ExecutorsActive.Add(delta)
}

// Helper function to update queue metrics
func UpdateQueueMetrics(pending, running, completed, failed, cancelled map[string]int) {
	// Clear existing metrics
	JobsInQueue.Reset()
	
	// Update pending jobs by priority
	for priority, count := range pending {
		JobsInQueue.WithLabelValues("pending", priority).Set(float64(count))
	}
	
	// Update running jobs
	total := 0
	for _, count := range running {
		total += count
	}
	JobsInQueue.WithLabelValues("running", "all").Set(float64(total))
	
	// Update completed jobs
	total = 0
	for _, count := range completed {
		total += count
	}
	JobsInQueue.WithLabelValues("completed", "all").Set(float64(total))
	
	// Update failed jobs
	total = 0
	for _, count := range failed {
		total += count
	}
	JobsInQueue.WithLabelValues("failed", "all").Set(float64(total))
	
	// Update cancelled jobs
	total = 0
	for _, count := range cancelled {
		total += count
	}
	JobsInQueue.WithLabelValues("cancelled", "all").Set(float64(total))
}
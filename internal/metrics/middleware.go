package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPMiddleware wraps an http.Handler to collect metrics
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Normalize the endpoint for metrics (remove IDs)
		endpoint := normalizeEndpoint(r.URL.Path)
		
		// Wrap the response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		
		// Call the next handler
		next.ServeHTTP(wrapped, r)
		
		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrapped.statusCode)
		
		APIRequests.WithLabelValues(r.Method, endpoint, status).Inc()
		APIRequestDuration.WithLabelValues(r.Method, endpoint).Observe(duration)
	})
}

// normalizeEndpoint removes IDs from paths for consistent metrics
func normalizeEndpoint(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		// Check if part looks like a UUID or numeric ID
		if isID(part) {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

// isID checks if a string is likely an ID (UUID or numeric)
func isID(s string) bool {
	if len(s) == 0 {
		return false
	}
	
	// Check for UUID format (with dashes)
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return true
	}
	
	// Check if it's all digits
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *responseWriter) WriteHeader(statusCode int) {
	if !w.written {
		w.statusCode = statusCode
		w.written = true
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// PrometheusHandler returns the Prometheus metrics handler
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}
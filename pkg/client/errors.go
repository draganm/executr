package client

import (
	"errors"
	"fmt"
	"net/http"
)

// Common errors
var (
	// ErrJobNotFound indicates that the requested job was not found
	ErrJobNotFound = errors.New("job not found")
	
	// ErrNoJobsAvailable indicates that no jobs are available to claim
	ErrNoJobsAvailable = errors.New("no jobs available")
	
	// ErrUnauthorized indicates that the request was unauthorized
	ErrUnauthorized = errors.New("unauthorized")
	
	// ErrServerError indicates a server-side error
	ErrServerError = errors.New("server error")
	
	// ErrBadRequest indicates a malformed request
	ErrBadRequest = errors.New("bad request")
	
	// ErrNetworkError indicates a network-related error
	ErrNetworkError = errors.New("network error")
)

// APIError represents a detailed error from the API
type APIError struct {
	StatusCode int
	Message    string
	Context    map[string]interface{}
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Context != nil {
		return fmt.Sprintf("API error (status %d): %s, context: %v", e.StatusCode, e.Message, e.Context)
	}
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound checks if the error indicates a not found condition
func IsNotFound(err error) bool {
	if errors.Is(err, ErrJobNotFound) {
		return true
	}
	
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	
	return false
}

// IsNoJobsAvailable checks if the error indicates no jobs are available
func IsNoJobsAvailable(err error) bool {
	return errors.Is(err, ErrNoJobsAvailable)
}

// IsServerError checks if the error is a server-side error
func IsServerError(err error) bool {
	if errors.Is(err, ErrServerError) {
		return true
	}
	
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500
	}
	
	return false
}

// IsBadRequest checks if the error is due to a bad request
func IsBadRequest(err error) bool {
	if errors.Is(err, ErrBadRequest) {
		return true
	}
	
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusBadRequest
	}
	
	return false
}

// IsNetworkError checks if the error is network-related
func IsNetworkError(err error) bool {
	return errors.Is(err, ErrNetworkError)
}
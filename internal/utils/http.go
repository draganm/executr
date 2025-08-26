package utils

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// RetryableHTTPClient is an HTTP client with retry logic
type RetryableHTTPClient struct {
	client      *http.Client
	maxRetries  int
	retryDelay  time.Duration
	maxDelay    time.Duration
	shouldRetry func(resp *http.Response, err error) bool
}

// NewRetryableHTTPClient creates a new HTTP client with retry logic
func NewRetryableHTTPClient() *RetryableHTTPClient {
	return &RetryableHTTPClient{
		client:     &http.Client{Timeout: 30 * time.Second},
		maxRetries: 3,
		retryDelay: 1 * time.Second,
		maxDelay:   10 * time.Second,
		shouldRetry: func(resp *http.Response, err error) bool {
			if err != nil {
				return true
			}
			// Retry on 5xx errors and 429 (Too Many Requests)
			return resp.StatusCode >= 500 || resp.StatusCode == 429
		},
	}
}

// Do executes an HTTP request with retry logic
func (c *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithContext(req.Context(), req)
}

// DoWithContext executes an HTTP request with retry logic and context
func (c *RetryableHTTPClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	
	delay := c.retryDelay
	
	for i := 0; i <= c.maxRetries; i++ {
		// Clone the request for each attempt
		reqCopy := req.Clone(ctx)
		
		resp, err = c.client.Do(reqCopy)
		
		// Check if we should retry
		if !c.shouldRetry(resp, err) {
			return resp, err
		}
		
		// Don't retry if it's the last attempt
		if i == c.maxRetries {
			break
		}
		
		// Close the response body if it exists
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		
		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Exponential backoff with max delay
			delay = delay * 2
			if delay > c.maxDelay {
				delay = c.maxDelay
			}
		}
	}
	
	if err != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.maxRetries, err)
	}
	
	return resp, nil
}

// SetMaxRetries sets the maximum number of retries
func (c *RetryableHTTPClient) SetMaxRetries(n int) {
	c.maxRetries = n
}

// SetTimeout sets the HTTP client timeout
func (c *RetryableHTTPClient) SetTimeout(timeout time.Duration) {
	c.client.Timeout = timeout
}
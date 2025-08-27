package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/draganm/executr/internal/models"
	"github.com/draganm/executr/internal/utils"
)

// Client is the interface for interacting with the Executr server
type Client interface {
	// SubmitJob submits a new job to the server
	SubmitJob(ctx context.Context, job *models.JobSubmission) (*models.Job, error)
	
	// GetJob retrieves a job by ID
	GetJob(ctx context.Context, jobID uuid.UUID) (*models.Job, error)
	
	// ListJobs lists jobs with optional filtering
	ListJobs(ctx context.Context, filter *ListJobsFilter) ([]*models.Job, error)
	
	// CancelJob cancels a pending job
	CancelJob(ctx context.Context, jobID uuid.UUID) error
	
	// ClaimNextJob claims the next available job for an executor
	ClaimNextJob(ctx context.Context, executorID, executorIP string) (*models.Job, error)
	
	// Heartbeat sends a heartbeat for a running job
	Heartbeat(ctx context.Context, jobID uuid.UUID, executorID string) error
	
	// CompleteJob marks a job as completed
	CompleteJob(ctx context.Context, jobID uuid.UUID, result *models.CompleteRequest) error
	
	// FailJob marks a job as failed
	FailJob(ctx context.Context, jobID uuid.UUID, result *models.FailRequest) error
	
	// Health checks the server health
	Health(ctx context.Context) (*HealthResponse, error)
}

// ListJobsFilter contains filtering options for listing jobs
type ListJobsFilter struct {
	Status   string
	Type     string
	Priority string
	Limit    int
	Offset   int
}

// HealthResponse represents the server health status
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// ErrorResponse represents an error response from the server
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// HTTPClient implements the Client interface using HTTP
type HTTPClient struct {
	baseURL    string
	httpClient *utils.RetryableHTTPClient
}

// New creates a new HTTP client for the Executr server (simplified alias)
func New(baseURL string) Client {
	return NewClient(baseURL)
}

// NewClient creates a new HTTP client for the Executr server
func NewClient(baseURL string) Client {
	// Ensure baseURL doesn't end with a slash
	baseURL = strings.TrimRight(baseURL, "/")
	
	return &HTTPClient{
		baseURL:    baseURL,
		httpClient: utils.NewRetryableHTTPClient(),
	}
}

// NewClientWithOptions creates a new HTTP client with custom options
func NewClientWithOptions(baseURL string, maxRetries int, timeout time.Duration) Client {
	baseURL = strings.TrimRight(baseURL, "/")
	
	httpClient := utils.NewRetryableHTTPClient()
	httpClient.SetMaxRetries(maxRetries)
	httpClient.SetTimeout(timeout)
	
	return &HTTPClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// SubmitJob submits a new job to the server
func (c *HTTPClient) SubmitJob(ctx context.Context, job *models.JobSubmission) (*models.Job, error) {
	body, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/jobs", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var result models.Job
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetJob retrieves a job by ID
func (c *HTTPClient) GetJob(ctx context.Context, jobID uuid.UUID) (*models.Job, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/jobs/"+jobID.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result models.Job
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListJobs lists jobs with optional filtering
func (c *HTTPClient) ListJobs(ctx context.Context, filter *ListJobsFilter) ([]*models.Job, error) {
	params := url.Values{}
	if filter != nil {
		if filter.Status != "" {
			params.Set("status", filter.Status)
		}
		if filter.Type != "" {
			params.Set("type", filter.Type)
		}
		if filter.Priority != "" {
			params.Set("priority", filter.Priority)
		}
		if filter.Limit > 0 {
			params.Set("limit", strconv.Itoa(filter.Limit))
		}
		if filter.Offset > 0 {
			params.Set("offset", strconv.Itoa(filter.Offset))
		}
	}

	reqURL := c.baseURL + "/api/v1/jobs"
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result []*models.Job
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// CancelJob cancels a pending job
func (c *HTTPClient) CancelJob(ctx context.Context, jobID uuid.UUID) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/v1/jobs/"+jobID.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}

	return nil
}

// ClaimNextJob claims the next available job for an executor
func (c *HTTPClient) ClaimNextJob(ctx context.Context, executorID, executorIP string) (*models.Job, error) {
	claim := models.ClaimRequest{
		ExecutorID: executorID,
		ExecutorIP: executorIP,
	}

	body, err := json.Marshal(claim)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal claim request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/jobs/claim", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// No content means no jobs available
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result models.Job
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Heartbeat sends a heartbeat for a running job
func (c *HTTPClient) Heartbeat(ctx context.Context, jobID uuid.UUID, executorID string) error {
	heartbeat := models.HeartbeatRequest{
		ExecutorID: executorID,
	}

	body, err := json.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+"/api/v1/jobs/"+jobID.String()+"/heartbeat", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}

	return nil
}

// CompleteJob marks a job as completed
func (c *HTTPClient) CompleteJob(ctx context.Context, jobID uuid.UUID, result *models.CompleteRequest) error {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal complete request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+"/api/v1/jobs/"+jobID.String()+"/complete", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}

	return nil
}

// FailJob marks a job as failed
func (c *HTTPClient) FailJob(ctx context.Context, jobID uuid.UUID, result *models.FailRequest) error {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal fail request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+"/api/v1/jobs/"+jobID.String()+"/fail", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}

	return nil
}

// Health checks the server health
func (c *HTTPClient) Health(ctx context.Context) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/health", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.DoWithContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// parseError parses an error response from the server
func (c *HTTPClient) parseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		// If we can't parse the error, return the raw body
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	if errResp.Context != nil {
		return fmt.Errorf("%s (context: %v)", errResp.Error, errResp.Context)
	}
	return fmt.Errorf("%s", errResp.Error)
}
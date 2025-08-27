package client

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/draganm/executr/internal/models"
)

// MockClient is a mock implementation of the Client interface for testing
type MockClient struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]*models.Job

	// Configurable behavior
	SubmitJobFunc      func(ctx context.Context, job *models.JobSubmission) (*models.Job, error)
	GetJobFunc         func(ctx context.Context, jobID uuid.UUID) (*models.Job, error)
	ListJobsFunc       func(ctx context.Context, filter *ListJobsFilter) ([]*models.Job, error)
	CancelJobFunc      func(ctx context.Context, jobID uuid.UUID) error
	ClaimNextJobFunc   func(ctx context.Context, executorID, executorIP string) (*models.Job, error)
	HeartbeatFunc      func(ctx context.Context, jobID uuid.UUID, executorID string) error
	CompleteJobFunc    func(ctx context.Context, jobID uuid.UUID, result *models.CompleteRequest) error
	FailJobFunc        func(ctx context.Context, jobID uuid.UUID, result *models.FailRequest) error
	HealthFunc         func(ctx context.Context) (*HealthResponse, error)
}

// NewMockClient creates a new mock client
func NewMockClient() *MockClient {
	return &MockClient{
		jobs: make(map[uuid.UUID]*models.Job),
	}
}

// SubmitJob submits a new job
func (m *MockClient) SubmitJob(ctx context.Context, submission *models.JobSubmission) (*models.Job, error) {
	if m.SubmitJobFunc != nil {
		return m.SubmitJobFunc(ctx, submission)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job := &models.Job{
		ID:           uuid.New(),
		Type:         submission.Type,
		BinaryURL:    submission.BinaryURL,
		BinarySHA256: submission.BinarySHA256,
		Arguments:    submission.Arguments,
		EnvVariables: submission.EnvVariables,
		Priority:     submission.Priority,
		Status:       models.StatusPending,
	}

	m.jobs[job.ID] = job
	return job, nil
}

// GetJob retrieves a job by ID
func (m *MockClient) GetJob(ctx context.Context, jobID uuid.UUID) (*models.Job, error) {
	if m.GetJobFunc != nil {
		return m.GetJobFunc(ctx, jobID)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, ErrJobNotFound
	}

	return job, nil
}

// ListJobs lists all jobs with optional filtering
func (m *MockClient) ListJobs(ctx context.Context, filter *ListJobsFilter) ([]*models.Job, error) {
	if m.ListJobsFunc != nil {
		return m.ListJobsFunc(ctx, filter)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.Job
	for _, job := range m.jobs {
		// Apply filters if provided
		if filter != nil {
			if filter.Status != "" && string(job.Status) != filter.Status {
				continue
			}
			if filter.Type != "" && job.Type != filter.Type {
				continue
			}
			if filter.Priority != "" && string(job.Priority) != filter.Priority {
				continue
			}
		}
		result = append(result, job)
	}

	// Apply limit and offset
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(result) {
			result = result[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(result) {
			result = result[:filter.Limit]
		}
	}

	return result, nil
}

// CancelJob cancels a pending job
func (m *MockClient) CancelJob(ctx context.Context, jobID uuid.UUID) error {
	if m.CancelJobFunc != nil {
		return m.CancelJobFunc(ctx, jobID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status != models.StatusPending {
		return ErrBadRequest
	}

	job.Status = models.StatusCancelled
	return nil
}

// ClaimNextJob claims the next available job
func (m *MockClient) ClaimNextJob(ctx context.Context, executorID, executorIP string) (*models.Job, error) {
	if m.ClaimNextJobFunc != nil {
		return m.ClaimNextJobFunc(ctx, executorID, executorIP)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the next pending job
	for _, job := range m.jobs {
		if job.Status == models.StatusPending {
			job.Status = models.StatusRunning
			job.ExecutorID = executorID
			return job, nil
		}
	}

	return nil, nil // No jobs available
}

// Heartbeat sends a heartbeat for a running job
func (m *MockClient) Heartbeat(ctx context.Context, jobID uuid.UUID, executorID string) error {
	if m.HeartbeatFunc != nil {
		return m.HeartbeatFunc(ctx, jobID, executorID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status != models.StatusRunning {
		return ErrBadRequest
	}

	if job.ExecutorID != executorID {
		return ErrUnauthorized
	}

	return nil
}

// CompleteJob marks a job as completed
func (m *MockClient) CompleteJob(ctx context.Context, jobID uuid.UUID, result *models.CompleteRequest) error {
	if m.CompleteJobFunc != nil {
		return m.CompleteJobFunc(ctx, jobID, result)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status != models.StatusRunning {
		return ErrBadRequest
	}

	if job.ExecutorID != result.ExecutorID {
		return ErrUnauthorized
	}

	job.Status = models.StatusCompleted
	job.Stdout = result.Stdout
	job.Stderr = result.Stderr
	exitCode := result.ExitCode
	job.ExitCode = &exitCode

	return nil
}

// FailJob marks a job as failed
func (m *MockClient) FailJob(ctx context.Context, jobID uuid.UUID, result *models.FailRequest) error {
	if m.FailJobFunc != nil {
		return m.FailJobFunc(ctx, jobID, result)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status != models.StatusRunning {
		return ErrBadRequest
	}

	if job.ExecutorID != result.ExecutorID {
		return ErrUnauthorized
	}

	job.Status = models.StatusFailed
	job.ErrorMessage = result.ErrorMessage
	job.Stdout = result.Stdout
	job.Stderr = result.Stderr
	if result.ExitCode != 0 {
		exitCode := result.ExitCode
		job.ExitCode = &exitCode
	}

	return nil
}

// Health checks the server health
func (m *MockClient) Health(ctx context.Context) (*HealthResponse, error) {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}

	return &HealthResponse{
		Status:   "healthy",
		Database: "connected",
	}, nil
}

// AddJob adds a job to the mock client's storage
func (m *MockClient) AddJob(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
}

// GetAllJobs returns all jobs in the mock client's storage
func (m *MockClient) GetAllJobs() map[uuid.UUID]*models.Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[uuid.UUID]*models.Job)
	for k, v := range m.jobs {
		result[k] = v
	}
	return result
}

// ClearJobs clears all jobs from the mock client's storage
func (m *MockClient) ClearJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = make(map[uuid.UUID]*models.Job)
}
package models

import (
	"time"

	"github.com/google/uuid"
)

// Priority represents job execution priority
type Priority string

const (
	PriorityForeground Priority = "foreground"
	PriorityBackground Priority = "background"
	PriorityBestEffort Priority = "best_effort"
)

// Status represents job execution status
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Job represents a job in the system
type Job struct {
	ID            uuid.UUID              `json:"id"`
	Type          string                 `json:"type"`
	BinaryURL     string                 `json:"binary_url"`
	BinarySHA256  string                 `json:"binary_sha256"`
	Arguments     []string               `json:"arguments,omitempty"`
	EnvVariables  map[string]string      `json:"env_variables,omitempty"`
	Priority      Priority               `json:"priority"`
	Status        Status                 `json:"status"`
	ExecutorID    string                 `json:"executor_id,omitempty"`
	Stdout        string                 `json:"stdout,omitempty"`
	Stderr        string                 `json:"stderr,omitempty"`
	ExitCode      *int                   `json:"exit_code,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	StartedAt     *time.Time             `json:"started_at,omitempty"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	LastHeartbeat *time.Time             `json:"last_heartbeat,omitempty"`
}

// JobResult represents the result of a job execution
type JobResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// JobAttempt represents a single execution attempt of a job
type JobAttempt struct {
	ID           uuid.UUID  `json:"id"`
	JobID        uuid.UUID  `json:"job_id"`
	ExecutorID   string     `json:"executor_id"`
	ExecutorIP   string     `json:"executor_ip"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// JobSubmission represents a job submission request
type JobSubmission struct {
	Type         string            `json:"type"`
	BinaryURL    string            `json:"binary_url"`
	BinarySHA256 string            `json:"binary_sha256,omitempty"`
	Arguments    []string          `json:"arguments,omitempty"`
	EnvVariables map[string]string `json:"env_variables,omitempty"`
	Priority     Priority          `json:"priority"`
	MaxRetries   int               `json:"max_retries,omitempty"`
}

// ClaimRequest represents a job claim request from an executor
type ClaimRequest struct {
	ExecutorID string `json:"executor_id"`
	ExecutorIP string `json:"executor_ip"`
}

// HeartbeatRequest represents a heartbeat update from an executor
type HeartbeatRequest struct {
	ExecutorID string `json:"executor_id"`
}

// CompleteRequest represents a job completion request
type CompleteRequest struct {
	ExecutorID string `json:"executor_id"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
}

// FailRequest represents a job failure request
type FailRequest struct {
	ExecutorID   string `json:"executor_id"`
	ErrorMessage string `json:"error_message"`
	Stdout       string `json:"stdout,omitempty"`
	Stderr       string `json:"stderr,omitempty"`
	ExitCode     int    `json:"exit_code,omitempty"`
}
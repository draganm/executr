package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/draganm/executr/internal/models"
)

const (
	maxOutputSize = 1024 * 1024 // 1MB
	maxHeadLines  = 500
)

type JobRunner struct {
	JobID      string
	BinaryPath string
	Arguments  []string
	EnvVars    map[string]string
	WorkDir    string
}

func (r *JobRunner) Execute(ctx context.Context) *models.JobResult {
	slog.Info("Executing job",
		"job_id", r.JobID,
		"binary", r.BinaryPath,
		"work_dir", r.WorkDir,
		"args", r.Arguments,
	)
	
	// Create command with arguments passed separately
	cmd := exec.CommandContext(ctx, r.BinaryPath, r.Arguments...)
	
	// Set working directory
	cmd.Dir = r.WorkDir
	
	// Replace environment completely with job's env variables
	if len(r.EnvVars) > 0 {
		env := make([]string, 0, len(r.EnvVars))
		for key, value := range r.EnvVars {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		cmd.Env = env
	} else {
		// Use empty environment if no env vars specified
		cmd.Env = []string{}
	}
	
	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Run the command
	err := cmd.Run()
	
	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Command couldn't be started or other error
			exitCode = -1
			stderr.WriteString(fmt.Sprintf("\nExecution error: %v", err))
		}
	}
	
	// Truncate output if necessary
	stdoutStr := truncateOutput(stdout.String())
	stderrStr := truncateOutput(stderr.String())
	
	result := &models.JobResult{
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		ExitCode: exitCode,
	}
	
	slog.Info("Job execution completed",
		"job_id", r.JobID,
		"exit_code", exitCode,
		"stdout_size", len(stdoutStr),
		"stderr_size", len(stderrStr),
	)
	
	return result
}

func truncateOutput(output string) string {
	if len(output) <= maxOutputSize {
		return output
	}
	
	lines := strings.Split(output, "\n")
	
	// If we have fewer lines than maxHeadLines, just truncate by bytes
	if len(lines) <= maxHeadLines {
		return output[:maxOutputSize]
	}
	
	// Keep first maxHeadLines
	result := strings.Join(lines[:maxHeadLines], "\n")
	
	// Add truncation marker
	truncMarker := fmt.Sprintf("\n... [OUTPUT TRUNCATED - Total %d bytes, %d lines] ...\n", 
		len(output), len(lines))
	result += truncMarker
	
	// Calculate how much space we have left
	remaining := maxOutputSize - len(result)
	if remaining <= 0 {
		return result[:maxOutputSize]
	}
	
	// Add as many lines from the end as fit
	tailLines := []string{}
	tailSize := 0
	
	for i := len(lines) - 1; i >= maxHeadLines; i-- {
		lineSize := len(lines[i]) + 1 // +1 for newline
		if tailSize+lineSize > remaining {
			break
		}
		tailLines = append([]string{lines[i]}, tailLines...)
		tailSize += lineSize
	}
	
	if len(tailLines) > 0 {
		result += strings.Join(tailLines, "\n")
	}
	
	return result
}
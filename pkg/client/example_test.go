package client_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/draganm/executr/internal/models"
	"github.com/draganm/executr/pkg/client"
)

func ExampleClient() {
	// Create a new client
	c := client.NewClient("http://localhost:8080")

	ctx := context.Background()

	// Submit a job
	job, err := c.SubmitJob(ctx, &models.JobSubmission{
		Type:         "example-job",
		BinaryURL:    "https://example.com/binary",
		BinarySHA256: "abc123def456",
		Arguments:    []string{"--input", "data.txt"},
		EnvVariables: map[string]string{
			"ENV_VAR": "value",
		},
		Priority: models.PriorityBackground,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Submitted job: %s\n", job.ID)

	// Get job status
	job, err = c.GetJob(ctx, job.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Job status: %s\n", job.Status)

	// List jobs
	jobs, err := c.ListJobs(ctx, &client.ListJobsFilter{
		Status: "pending",
		Limit:  10,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d pending jobs\n", len(jobs))
}

func ExampleClient_executor() {
	// Create a new client
	c := client.NewClient("http://localhost:8080")

	ctx := context.Background()
	executorID := "worker-1"
	executorIP := "192.168.1.100"

	// Claim a job
	job, err := c.ClaimNextJob(ctx, executorID, executorIP)
	if err != nil {
		log.Fatal(err)
	}

	if job == nil {
		fmt.Println("No jobs available")
		return
	}

	fmt.Printf("Claimed job: %s\n", job.ID)

	// Send heartbeats while processing
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if err := c.Heartbeat(ctx, job.ID, executorID); err != nil {
					log.Printf("Heartbeat failed: %v", err)
				}
			}
		}
	}()

	// Simulate job execution
	time.Sleep(10 * time.Second)

	// Stop heartbeats
	cancelHeartbeat()

	// Mark job as completed
	err = c.CompleteJob(ctx, job.ID, &models.CompleteRequest{
		ExecutorID: executorID,
		Stdout:     "Job completed successfully",
		Stderr:     "",
		ExitCode:   0,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Job completed")
}

func ExampleClient_errorHandling() {
	c := client.NewClient("http://localhost:8080")

	ctx := context.Background()

	// Try to get a non-existent job
	_, err := c.GetJob(ctx, uuid.New())
	if err != nil {
		if client.IsNotFound(err) {
			fmt.Println("Job not found")
		} else if client.IsServerError(err) {
			fmt.Println("Server error occurred")
		} else {
			fmt.Printf("Unexpected error: %v\n", err)
		}
	}

	// Try to claim a job when none are available
	job, err := c.ClaimNextJob(ctx, "worker-1", "192.168.1.100")
	if err != nil {
		log.Fatal(err)
	}
	if job == nil {
		fmt.Println("No jobs available to claim")
	}
}
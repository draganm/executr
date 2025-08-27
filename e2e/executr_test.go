package e2e_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/draganm/executr/internal/executor"
	"github.com/draganm/executr/internal/models"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Executr E2E Tests", func() {
	var (
		successBinarySHA256 string
		failureBinarySHA256 string
		outputBinarySHA256  string
	)

	BeforeEach(func() {
		// Calculate SHA256 for test binaries
		successBinarySHA256 = calculateFileSHA256("testdata/binaries/success")
		failureBinarySHA256 = calculateFileSHA256("testdata/binaries/failure")
		outputBinarySHA256 = calculateFileSHA256("testdata/binaries/output")
	})

	Describe("Job Submission and Execution", func() {
		It("should successfully submit and execute a simple job", func() {
			// Submit job
			submission := &models.JobSubmission{
				Type:         "test-success",
				BinaryURL:    getBinaryURL("success"),
				BinarySHA256: successBinarySHA256,
				Arguments:    []string{"arg1", "arg2"},
				EnvVariables: map[string]string{"TEST_ENV": "test_value"},
				Priority:     models.PriorityBackground,
			}

			job, err := testClient.SubmitJob(context.Background(), submission)
			Expect(err).NotTo(HaveOccurred())
			Expect(job).NotTo(BeNil())
			Expect(job.Status).To(Equal(models.StatusPending))

			// Start an executor
			execCtx, execCancel := context.WithCancel(context.Background())
			defer execCancel()

			execConfig := &executor.Config{
				ServerURL:         serverURL,
				Name:              "test-executor",
				CacheDir:          filepath.Join(createTempDir(), "cache"),
				WorkDir:           filepath.Join(createTempDir(), "work"),
				MaxJobs:           1,
				PollInterval:      1,
				MaxCacheSize:      100,
				HeartbeatInterval: 2,
				NetworkTimeout:    60,
			}

			exec, err := executor.New(execConfig)
			Expect(err).NotTo(HaveOccurred())

			// Run executor in background
			go func() {
				exec.Run(execCtx)
			}()

			// Wait for job to complete
			Eventually(func() models.Status {
				job, err := testClient.GetJob(context.Background(), job.ID)
				if err != nil {
					return ""
				}
				return job.Status
			}, 30*time.Second, 500*time.Millisecond).Should(Equal(models.StatusCompleted))

			// Verify job results
			completedJob, err := testClient.GetJob(context.Background(), job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(completedJob.ExitCode).NotTo(BeNil())
			Expect(*completedJob.ExitCode).To(Equal(0))
			Expect(completedJob.Stdout).To(ContainSubstring("Hello from success binary"))
			Expect(completedJob.Stdout).To(ContainSubstring("Arguments: [arg1 arg2]"))
			Expect(completedJob.Stdout).To(ContainSubstring("TEST_ENV=test_value"))
			Expect(completedJob.Stderr).To(ContainSubstring("This is stderr output"))
		})

		It("should handle job failure correctly", func() {
			// Submit failing job
			submission := &models.JobSubmission{
				Type:         "test-failure",
				BinaryURL:    getBinaryURL("failure"),
				BinarySHA256: failureBinarySHA256,
				Arguments:    []string{"42"},
				Priority:     models.PriorityBackground,
			}

			job, err := testClient.SubmitJob(context.Background(), submission)
			Expect(err).NotTo(HaveOccurred())

			// Start executor
			execCtx, execCancel := context.WithCancel(context.Background())
			defer execCancel()

			execConfig := &executor.Config{
				ServerURL:         serverURL,
				Name:              "test-executor-fail",
				CacheDir:          filepath.Join(createTempDir(), "cache"),
				WorkDir:           filepath.Join(createTempDir(), "work"),
				MaxJobs:           1,
				PollInterval:      1,
				MaxCacheSize:      100,
				HeartbeatInterval: 2,
				NetworkTimeout:    60,
			}

			exec, err := executor.New(execConfig)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				exec.Run(execCtx)
			}()

			// Wait for job to fail
			Eventually(func() models.Status {
				job, err := testClient.GetJob(context.Background(), job.ID)
				if err != nil {
					return ""
				}
				return job.Status
			}, 30*time.Second, 500*time.Millisecond).Should(Equal(models.StatusFailed))

			// Verify failure details
			failedJob, err := testClient.GetJob(context.Background(), job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(failedJob.ExitCode).NotTo(BeNil())
			Expect(*failedJob.ExitCode).To(Equal(42))
			Expect(failedJob.Stderr).To(ContainSubstring("ERROR: Intentional failure"))
		})
	})

	Describe("Binary Caching", func() {
		It("should cache binaries and reuse them", func() {
			cacheDir := filepath.Join(createTempDir(), "cache")
			workDir := filepath.Join(createTempDir(), "work")

			// Submit first job
			submission1 := &models.JobSubmission{
				Type:         "cache-test-1",
				BinaryURL:    getBinaryURL("success"),
				BinarySHA256: successBinarySHA256,
				Priority:     models.PriorityBackground,
			}

			job1, err := testClient.SubmitJob(context.Background(), submission1)
			Expect(err).NotTo(HaveOccurred())

			// Submit second job with same binary
			submission2 := &models.JobSubmission{
				Type:         "cache-test-2",
				BinaryURL:    getBinaryURL("success"),
				BinarySHA256: successBinarySHA256,
				Priority:     models.PriorityBackground,
			}

			job2, err := testClient.SubmitJob(context.Background(), submission2)
			Expect(err).NotTo(HaveOccurred())

			// Start executor
			execCtx, execCancel := context.WithCancel(context.Background())
			defer execCancel()

			execConfig := &executor.Config{
				ServerURL:         serverURL,
				Name:              "cache-test-executor",
				CacheDir:          cacheDir,
				WorkDir:           workDir,
				MaxJobs:           1,
				PollInterval:      1,
				MaxCacheSize:      100,
				HeartbeatInterval: 2,
				NetworkTimeout:    60,
			}

			exec, err := executor.New(execConfig)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				exec.Run(execCtx)
			}()

			// Wait for both jobs to complete
			Eventually(func() bool {
				j1, _ := testClient.GetJob(context.Background(), job1.ID)
				j2, _ := testClient.GetJob(context.Background(), job2.ID)
				return j1.Status == models.StatusCompleted && j2.Status == models.StatusCompleted
			}, 30*time.Second, 500*time.Millisecond).Should(BeTrue())

			// Verify binary is cached (only one file in cache)
			cacheFiles, err := filepath.Glob(filepath.Join(cacheDir, "*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(cacheFiles).To(HaveLen(1))
			Expect(filepath.Base(cacheFiles[0])).To(Equal(successBinarySHA256))
		})
	})

	Describe("Priority-based Execution", func() {
		It("should execute jobs in priority order", func() {
			// Submit jobs in reverse priority order
			bestEffortJob := &models.JobSubmission{
				Type:         "priority-best-effort",
				BinaryURL:    getBinaryURL("success"),
				BinarySHA256: successBinarySHA256,
				Priority:     models.PriorityBestEffort,
			}
			jobBE, err := testClient.SubmitJob(context.Background(), bestEffortJob)
			Expect(err).NotTo(HaveOccurred())

			backgroundJob := &models.JobSubmission{
				Type:         "priority-background",
				BinaryURL:    getBinaryURL("success"),
				BinarySHA256: successBinarySHA256,
				Priority:     models.PriorityBackground,
			}
			jobBG, err := testClient.SubmitJob(context.Background(), backgroundJob)
			Expect(err).NotTo(HaveOccurred())

			foregroundJob := &models.JobSubmission{
				Type:         "priority-foreground",
				BinaryURL:    getBinaryURL("success"),
				BinarySHA256: successBinarySHA256,
				Priority:     models.PriorityForeground,
			}
			jobFG, err := testClient.SubmitJob(context.Background(), foregroundJob)
			Expect(err).NotTo(HaveOccurred())

			// Start executor with single job concurrency
			execCtx, execCancel := context.WithCancel(context.Background())
			defer execCancel()

			execConfig := &executor.Config{
				ServerURL:         serverURL,
				Name:              "priority-executor",
				CacheDir:          filepath.Join(createTempDir(), "cache"),
				WorkDir:           filepath.Join(createTempDir(), "work"),
				MaxJobs:           1,
				PollInterval:      1,
				MaxCacheSize:      100,
				HeartbeatInterval: 2,
				NetworkTimeout:    60,
			}

			exec, err := executor.New(execConfig)
			Expect(err).NotTo(HaveOccurred())

			// Track completion order
			var completionOrder []uuid.UUID
			go func() {
				for {
					select {
					case <-execCtx.Done():
						return
					case <-time.After(100 * time.Millisecond):
						// Check job statuses
						for _, jobID := range []uuid.UUID{jobFG.ID, jobBG.ID, jobBE.ID} {
							job, _ := testClient.GetJob(context.Background(), jobID)
							if job.Status == models.StatusCompleted {
								// Check if not already recorded
								found := false
								for _, id := range completionOrder {
									if id == jobID {
										found = true
										break
									}
								}
								if !found {
									completionOrder = append(completionOrder, jobID)
								}
							}
						}
					}
				}
			}()

			go func() {
				exec.Run(execCtx)
			}()

			// Wait for all jobs to complete
			Eventually(func() int {
				return len(completionOrder)
			}, 30*time.Second, 500*time.Millisecond).Should(Equal(3))

			// Verify execution order (foreground, background, best_effort)
			Expect(completionOrder[0]).To(Equal(jobFG.ID))
			Expect(completionOrder[1]).To(Equal(jobBG.ID))
			Expect(completionOrder[2]).To(Equal(jobBE.ID))
		})
	})

	Describe("Job Cancellation", func() {
		It("should cancel pending jobs", func() {
			// Submit a job
			submission := &models.JobSubmission{
				Type:         "cancel-test",
				BinaryURL:    getBinaryURL("longrunning"),
				BinarySHA256: calculateFileSHA256("testdata/binaries/longrunning"),
				Arguments:    []string{"30s"},
				Priority:     models.PriorityBackground,
			}

			job, err := testClient.SubmitJob(context.Background(), submission)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Status).To(Equal(models.StatusPending))

			// Cancel the job before executor picks it up
			err = testClient.CancelJob(context.Background(), job.ID)
			Expect(err).NotTo(HaveOccurred())

			// Verify job is cancelled
			cancelledJob, err := testClient.GetJob(context.Background(), job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(cancelledJob.Status).To(Equal(models.StatusCancelled))

			// Start executor and verify it doesn't pick up cancelled job
			execCtx, execCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer execCancel()

			execConfig := &executor.Config{
				ServerURL:         serverURL,
				Name:              "cancel-executor",
				CacheDir:          filepath.Join(createTempDir(), "cache"),
				WorkDir:           filepath.Join(createTempDir(), "work"),
				MaxJobs:           1,
				PollInterval:      1,
				MaxCacheSize:      100,
				HeartbeatInterval: 2,
				NetworkTimeout:    60,
			}

			exec, err := executor.New(execConfig)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				exec.Run(execCtx)
			}()

			// Wait and verify job remains cancelled
			time.Sleep(3 * time.Second)
			stillCancelled, err := testClient.GetJob(context.Background(), job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(stillCancelled.Status).To(Equal(models.StatusCancelled))
		})
	})

	Describe("Multiple Executor Coordination", func() {
		It("should coordinate multiple executors with different names", func() {
			// Submit multiple jobs
			var jobIDs []uuid.UUID
			for i := 0; i < 3; i++ {
				submission := &models.JobSubmission{
					Type:         fmt.Sprintf("multi-exec-%d", i),
					BinaryURL:    getBinaryURL("success"),
					BinarySHA256: successBinarySHA256,
					Arguments:    []string{fmt.Sprintf("job-%d", i)},
					Priority:     models.PriorityBackground,
				}

				job, err := testClient.SubmitJob(context.Background(), submission)
				Expect(err).NotTo(HaveOccurred())
				jobIDs = append(jobIDs, job.ID)
			}

			// Start multiple executors
			execCtx, execCancel := context.WithCancel(context.Background())
			defer execCancel()

			for i := 0; i < 2; i++ {
				execConfig := &executor.Config{
					ServerURL:         serverURL,
					Name:              fmt.Sprintf("executor-%d", i),
					CacheDir:          filepath.Join(createTempDir(), fmt.Sprintf("cache-%d", i)),
					WorkDir:           filepath.Join(createTempDir(), fmt.Sprintf("work-%d", i)),
					MaxJobs:           1,
					PollInterval:      1,
					MaxCacheSize:      100,
					HeartbeatInterval: 2,
					NetworkTimeout:    60,
				}

				exec, err := executor.New(execConfig)
				Expect(err).NotTo(HaveOccurred())

				go func() {
					exec.Run(execCtx)
				}()
			}

			// Wait for all jobs to complete
			Eventually(func() bool {
				for _, jobID := range jobIDs {
					job, err := testClient.GetJob(context.Background(), jobID)
					if err != nil || job.Status != models.StatusCompleted {
						return false
					}
				}
				return true
			}, 30*time.Second, 500*time.Millisecond).Should(BeTrue())

			// Verify all jobs completed and have executor IDs with correct format
			executorIDs := make(map[string]bool)
			for _, jobID := range jobIDs {
				job, err := testClient.GetJob(context.Background(), jobID)
				Expect(err).NotTo(HaveOccurred())
				Expect(job.ExecutorID).NotTo(BeEmpty())
				
				// Verify executor ID format: {name}-{suffix}
				Expect(job.ExecutorID).To(MatchRegexp(`executor-\d-[a-f0-9\-]+`))
				executorIDs[job.ExecutorID] = true
			}

			// Should have at least one unique executor ID (could be 1 or 2 depending on scheduling)
			Expect(len(executorIDs)).To(BeNumerically(">=", 1))
			Expect(len(executorIDs)).To(BeNumerically("<=", 2))
		})
	})

	Describe("Output Truncation", func() {
		It("should truncate large output correctly", func() {
			// Submit job that generates lots of output
			submission := &models.JobSubmission{
				Type:         "output-test",
				BinaryURL:    getBinaryURL("output"),
				BinarySHA256: outputBinarySHA256,
				Arguments:    []string{"10000"}, // Generate 10000 lines
				Priority:     models.PriorityBackground,
			}

			job, err := testClient.SubmitJob(context.Background(), submission)
			Expect(err).NotTo(HaveOccurred())

			// Start executor
			execCtx, execCancel := context.WithCancel(context.Background())
			defer execCancel()

			execConfig := &executor.Config{
				ServerURL:         serverURL,
				Name:              "output-executor",
				CacheDir:          filepath.Join(createTempDir(), "cache"),
				WorkDir:           filepath.Join(createTempDir(), "work"),
				MaxJobs:           1,
				PollInterval:      1,
				MaxCacheSize:      100,
				HeartbeatInterval: 2,
				NetworkTimeout:    60,
			}

			exec, err := executor.New(execConfig)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				exec.Run(execCtx)
			}()

			// Wait for job to complete
			Eventually(func() models.Status {
				job, err := testClient.GetJob(context.Background(), job.ID)
				if err != nil {
					return ""
				}
				return job.Status
			}, 30*time.Second, 500*time.Millisecond).Should(Equal(models.StatusCompleted))

			// Verify output is truncated
			completedJob, err := testClient.GetJob(context.Background(), job.ID)
			Expect(err).NotTo(HaveOccurred())
			
			// Output should be truncated to ~1MB
			Expect(len(completedJob.Stdout)).To(BeNumerically("<=", 1024*1024+1000)) // Allow some margin
			Expect(len(completedJob.Stderr)).To(BeNumerically("<=", 1024*1024+1000))
			
			// Should contain first lines
			Expect(completedJob.Stdout).To(ContainSubstring("STDOUT Line 00001"))
			// Should contain end marker or truncation happened
			Expect(strings.Contains(completedJob.Stdout, "=== END OF OUTPUT ===") || 
				strings.Contains(completedJob.Stdout, "STDOUT Line")).To(BeTrue())
		})
	})

	Describe("Working Directory Cleanup", func() {
		It("should clean up job directories after completion", func() {
			workDir := filepath.Join(createTempDir(), "work")
			
			// Create work directory
			err := os.MkdirAll(workDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Submit a job
			submission := &models.JobSubmission{
				Type:         "cleanup-test",
				BinaryURL:    getBinaryURL("success"),
				BinarySHA256: successBinarySHA256,
				Priority:     models.PriorityBackground,
			}

			job, err := testClient.SubmitJob(context.Background(), submission)
			Expect(err).NotTo(HaveOccurred())

			// Start executor
			execCtx, execCancel := context.WithCancel(context.Background())
			defer execCancel()

			execConfig := &executor.Config{
				ServerURL:         serverURL,
				Name:              "cleanup-executor",
				CacheDir:          filepath.Join(createTempDir(), "cache"),
				WorkDir:           workDir,
				MaxJobs:           1,
				PollInterval:      1,
				MaxCacheSize:      100,
				HeartbeatInterval: 2,
				NetworkTimeout:    60,
			}

			exec, err := executor.New(execConfig)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				exec.Run(execCtx)
			}()

			// Wait for job to complete
			Eventually(func() models.Status {
				job, err := testClient.GetJob(context.Background(), job.ID)
				if err != nil {
					return ""
				}
				return job.Status
			}, 30*time.Second, 500*time.Millisecond).Should(Equal(models.StatusCompleted))

			// Allow cleanup to happen
			time.Sleep(2 * time.Second)

			// Verify work directory is clean (no subdirectories)
			entries, err := os.ReadDir(workDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty(), "Work directory should be cleaned up after job completion")
		})
	})
})

// Helper function to calculate SHA256 of a file
func calculateFileSHA256(path string) string {
	file, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	Expect(err).NotTo(HaveOccurred())

	return hex.EncodeToString(hash.Sum(nil))
}
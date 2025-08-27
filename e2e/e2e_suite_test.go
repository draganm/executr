package e2e_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/draganm/executr/internal/server"
	"github.com/draganm/executr/pkg/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Executr E2E Suite")
}

var (
	postgresContainer *postgres.PostgresContainer
	dbURL             string
	serverInstance    *server.Server
	serverURL         string
	binaryServer      *httptest.Server
	testClient        client.Client
	ctx               context.Context
	cancel            context.CancelFunc
)

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	// Build test binaries
	err := buildTestBinaries()
	Expect(err).NotTo(HaveOccurred())

	// Start PostgreSQL container
	postgresContainer, err = postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("executr_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Minute),
		),
	)
	Expect(err).NotTo(HaveOccurred())

	dbURL, err = postgresContainer.ConnectionString(ctx, "sslmode=disable")
	Expect(err).NotTo(HaveOccurred())

	// Verify database connection
	db, err := sql.Open("pgx", dbURL)
	Expect(err).NotTo(HaveOccurred())
	err = db.Ping()
	Expect(err).NotTo(HaveOccurred())
	db.Close()

	// Start test binary server
	binaryServer = httptest.NewServer(http.FileServer(http.Dir("testdata/binaries")))

	// Start executr server
	serverConfig := &server.Config{
		DatabaseURL:      dbURL,
		Port:             0, // Use random available port
		CleanupInterval:  3600,
		JobRetention:     172800,
		HeartbeatTimeout: 15,
		LogLevel:         "error",
	}

	serverInstance, err = server.New(serverConfig)
	Expect(err).NotTo(HaveOccurred())

	// Start server in background
	go func() {
		if err := serverInstance.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(2 * time.Second)

	// Get server URL
	serverURL = fmt.Sprintf("http://localhost:%d", serverInstance.Port())

	// Create test client
	testClient = client.New(serverURL)

	// Verify server is ready
	Eventually(func() error {
		resp, err := http.Get(serverURL + "/api/v1/health")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server not healthy: %d", resp.StatusCode)
		}
		return nil
	}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
})

var _ = AfterSuite(func() {
	// Cleanup
	if cancel != nil {
		cancel()
	}

	if binaryServer != nil {
		binaryServer.Close()
	}

	if postgresContainer != nil {
		err := postgresContainer.Terminate(context.Background())
		Expect(err).NotTo(HaveOccurred())
	}

	// Clean up test binaries
	cleanupTestBinaries()

	// Clean up test directories
	os.RemoveAll(filepath.Join(os.TempDir(), "executr-test-*"))
})

// Helper functions for tests
func getBinaryURL(filename string) string {
	return binaryServer.URL + "/" + filename
}

func createTempDir() string {
	dir, err := os.MkdirTemp("", "executr-test-*")
	Expect(err).NotTo(HaveOccurred())
	return dir
}

// buildTestBinaries builds all test binaries needed for E2E tests
func buildTestBinaries() error {
	binaries := []string{
		"success",
		"failure",
		"longrunning",
		"output",
	}

	for _, binary := range binaries {
		sourceFile := fmt.Sprintf("testdata/binaries/%s.go", binary)
		outputFile := fmt.Sprintf("testdata/binaries/%s", binary)
		
		cmd := exec.Command("go", "build", "-o", outputFile, sourceFile)
		cmd.Dir = "."
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build %s: %w\nOutput: %s", binary, err, output)
		}
	}
	
	return nil
}

// cleanupTestBinaries removes all compiled test binaries
func cleanupTestBinaries() {
	binaries := []string{
		"testdata/binaries/success",
		"testdata/binaries/failure",
		"testdata/binaries/longrunning",
		"testdata/binaries/output",
	}

	for _, binary := range binaries {
		os.Remove(binary)
	}
}
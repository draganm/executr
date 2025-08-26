package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ProgressFunc is a callback function for download progress
type ProgressFunc func(bytesDownloaded, totalBytes int64)

// DownloadOptions contains options for downloading binaries
type DownloadOptions struct {
	SHA256       string       // Expected SHA256 hash (optional)
	ProgressFunc ProgressFunc // Progress callback function (optional)
}

// BinaryDownloader handles binary downloads with progress tracking
type BinaryDownloader struct {
	client *RetryableHTTPClient
}

// NewBinaryDownloader creates a new binary downloader
func NewBinaryDownloader() *BinaryDownloader {
	client := NewRetryableHTTPClient()
	// No timeout for binary downloads (as per requirements)
	client.SetTimeout(0)
	return &BinaryDownloader{
		client: client,
	}
}

// Download downloads a binary from the given URL to the destination path
func (d *BinaryDownloader) Download(ctx context.Context, url, destPath string, opts *DownloadOptions) error {
	if opts == nil {
		opts = &DownloadOptions{}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.client.DoWithContext(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create temporary file in the same directory as destination
	tmpFile, err := os.CreateTemp(filepath.Dir(destPath), ".download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	
	// Clean up temp file on error
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	// Create a reader that tracks progress and calculates SHA256
	var reader io.Reader = resp.Body
	hasher := sha256.New()
	
	// Wrap with TeeReader to calculate hash while downloading
	reader = io.TeeReader(reader, hasher)
	
	// Wrap with progress reader if callback provided
	if opts.ProgressFunc != nil {
		reader = &progressReader{
			reader:       reader,
			totalBytes:   resp.ContentLength,
			progressFunc: opts.ProgressFunc,
		}
	}

	// Copy to temporary file
	if _, err = io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to save file: %w", err)
	}
	
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify SHA256 if provided
	if opts.SHA256 != "" {
		calculatedHash := hex.EncodeToString(hasher.Sum(nil))
		if calculatedHash != opts.SHA256 {
			return fmt.Errorf("SHA256 mismatch: expected %s, got %s", opts.SHA256, calculatedHash)
		}
	}

	// Set executable permissions
	if err = os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	// Atomically move to final destination
	if err = os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to move file to destination: %w", err)
	}

	return nil
}

// CalculateSHA256FromURL downloads and calculates SHA256 without saving the file
func (d *BinaryDownloader) CalculateSHA256FromURL(ctx context.Context, url string, progressFunc ProgressFunc) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.client.DoWithContext(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create reader with progress tracking
	var reader io.Reader = resp.Body
	if progressFunc != nil {
		reader = &progressReader{
			reader:       reader,
			totalBytes:   resp.ContentLength,
			progressFunc: progressFunc,
		}
	}

	// Calculate SHA256 while streaming
	hasher := sha256.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", fmt.Errorf("failed to calculate SHA256: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// progressReader wraps an io.Reader to track download progress
type progressReader struct {
	reader          io.Reader
	bytesDownloaded int64
	totalBytes      int64
	progressFunc    ProgressFunc
	lastUpdate      time.Time
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.bytesDownloaded += int64(n)
		
		// Update progress at most once per 100ms to avoid excessive callbacks
		now := time.Now()
		if now.Sub(r.lastUpdate) >= 100*time.Millisecond || err == io.EOF {
			r.progressFunc(r.bytesDownloaded, r.totalBytes)
			r.lastUpdate = now
		}
	}
	return n, err
}
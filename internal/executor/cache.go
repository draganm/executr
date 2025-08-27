package executor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/draganm/executr/internal/utils"
)

type BinaryCache struct {
	cacheDir     string
	maxSizeMB    int
	mu           sync.RWMutex
	entries      map[string]*cacheEntry
}

type cacheEntry struct {
	sha256     string
	path       string
	size       int64
	lastAccess time.Time
}

func NewBinaryCache(cacheDir string, maxSizeMB int) (*BinaryCache, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	
	cache := &BinaryCache{
		cacheDir:  cacheDir,
		maxSizeMB: maxSizeMB,
		entries:   make(map[string]*cacheEntry),
	}
	
	// Load existing cache entries
	if err := cache.loadEntries(); err != nil {
		slog.Warn("Failed to load cache entries", "error", err)
	}
	
	return cache, nil
}

func (c *BinaryCache) loadEntries() error {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return err
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			
			// Cache files are named by their SHA256 hash
			sha256Hash := entry.Name()
			c.entries[sha256Hash] = &cacheEntry{
				sha256:     sha256Hash,
				path:       filepath.Join(c.cacheDir, sha256Hash),
				size:       info.Size(),
				lastAccess: info.ModTime(),
			}
		}
	}
	
	return nil
}

func (c *BinaryCache) GetBinary(binaryURL, expectedSHA256 string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if binary is already in cache
	if entry, exists := c.entries[expectedSHA256]; exists {
		// Verify the cached binary still has correct SHA256
		if err := c.verifySHA256(entry.path, expectedSHA256); err == nil {
			// Update last access time
			entry.lastAccess = time.Now()
			os.Chtimes(entry.path, time.Now(), time.Now())
			
			slog.Debug("Binary found in cache", 
				"sha256", expectedSHA256,
				"path", entry.path,
			)
			return entry.path, nil
		}
		
		// SHA256 mismatch, remove from cache
		slog.Warn("Cached binary SHA256 mismatch, removing from cache",
			"expected", expectedSHA256,
			"path", entry.path,
		)
		delete(c.entries, expectedSHA256)
		os.Remove(entry.path)
	}
	
	// Download binary
	slog.Info("Downloading binary", 
		"url", binaryURL,
		"sha256", expectedSHA256,
	)
	
	cachePath := filepath.Join(c.cacheDir, expectedSHA256)
	tempPath := cachePath + ".tmp"
	
	// Download to temporary file
	if err := c.downloadBinary(binaryURL, tempPath); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	
	// Verify SHA256
	if err := c.verifySHA256(tempPath, expectedSHA256); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("SHA256 verification failed: %w", err)
	}
	
	// Make binary executable
	if err := os.Chmod(tempPath, 0755); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to make binary executable: %w", err)
	}
	
	// Move to final location
	if err := os.Rename(tempPath, cachePath); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to move binary to cache: %w", err)
	}
	
	// Get file info
	info, err := os.Stat(cachePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat cached binary: %w", err)
	}
	
	// Add to cache entries
	c.entries[expectedSHA256] = &cacheEntry{
		sha256:     expectedSHA256,
		path:       cachePath,
		size:       info.Size(),
		lastAccess: time.Now(),
	}
	
	// Perform LRU eviction if needed
	c.evictIfNeeded()
	
	slog.Info("Binary cached successfully",
		"sha256", expectedSHA256,
		"size", info.Size(),
	)
	
	return cachePath, nil
}

func (c *BinaryCache) downloadBinary(url, destPath string) error {
	// Create temporary file
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	
	// Download with retry logic
	downloader := &utils.Downloader{
		MaxRetries: 3,
		RetryDelay: time.Second,
	}
	
	return downloader.Download(url, out)
}

func (c *BinaryCache) verifySHA256(filePath, expectedSHA256 string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	
	actualSHA256 := hex.EncodeToString(hash.Sum(nil))
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
	}
	
	return nil
}

func (c *BinaryCache) evictIfNeeded() {
	// Calculate total cache size
	var totalSize int64
	for _, entry := range c.entries {
		totalSize += entry.size
	}
	
	maxBytes := int64(c.maxSizeMB) * 1024 * 1024
	
	if totalSize <= maxBytes {
		return
	}
	
	slog.Info("Cache size exceeded, performing LRU eviction",
		"current_size", totalSize,
		"max_size", maxBytes,
	)
	
	// Sort entries by last access time (oldest first)
	entries := make([]*cacheEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}
	
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].lastAccess.Before(entries[j].lastAccess)
	})
	
	// Evict oldest entries until we're under the limit
	for _, entry := range entries {
		if totalSize <= maxBytes {
			break
		}
		
		slog.Debug("Evicting cached binary",
			"sha256", entry.sha256,
			"size", entry.size,
			"last_access", entry.lastAccess,
		)
		
		if err := os.Remove(entry.path); err != nil {
			slog.Warn("Failed to remove cached binary",
				"path", entry.path,
				"error", err,
			)
		}
		
		delete(c.entries, entry.sha256)
		totalSize -= entry.size
	}
	
	slog.Info("Cache eviction complete",
		"new_size", totalSize,
		"entries", len(c.entries),
	)
}
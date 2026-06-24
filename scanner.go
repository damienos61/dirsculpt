// scanner.go
// Real-time directory scanning with async I/O
// Optimized for HDD with minimal memory footprint
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// ScanResult represents a filesystem entry
type ScanResult struct {
	Path      string
	Name      string
	Size      int64
	IsDir     bool
	ModTime   int64
	ParentDir string
	Error     error
}

// ScanStats tracks scanning progress
type ScanStats struct {
	FilesProcessed int64
	DirsProcessed  int64
	TotalSize      int64
	IsComplete     bool
}

var (
	scanStats ScanStats
	scanMutex sync.RWMutex
)

// ScanDirectory recursively scans directory structure asynchronously
func ScanDirectory(ctx context.Context, rootPath string, resultChan chan<- ScanResult, errChan chan<- error) {
	defer close(resultChan)

	// Semaphore to limit concurrent I/O operations
	const maxConcurrentOps = 4
	semaphore := make(chan struct{}, maxConcurrentOps)
	var wg sync.WaitGroup

	// Start recursive scan
	wg.Add(1)
	go scanRecursive(ctx, rootPath, "", &wg, semaphore, resultChan, errChan)

	// Wait for completion
	wg.Wait()

	// Mark scan as complete
	scanMutex.Lock()
	scanStats.IsComplete = true
	scanMutex.Unlock()
}

func scanRecursive(
	ctx context.Context,
	currentPath string,
	parentPath string,
	wg *sync.WaitGroup,
	semaphore chan struct{},
	resultChan chan<- ScanResult,
	errChan chan<- error,
) {
	defer wg.Done()

	// Acquire semaphore
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	// Check context
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Read directory entries
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		if os.IsPermission(err) {
			return // Skip permission denied
		}
		select {
		case errChan <- fmt.Errorf("failed to read directory %s: %w", currentPath, err):
		case <-ctx.Done():
		}
		return
	}

	for _, entry := range entries {
		// Respect context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Get entry info
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(currentPath, entry.Name())

		// Create result
		result := ScanResult{
			Path:      fullPath,
			Name:      entry.Name(),
			Size:      info.Size(),
			IsDir:     entry.IsDir(),
			ModTime:   info.ModTime().Unix(),
			ParentDir: currentPath,
		}

		// Send result
		select {
		case resultChan <- result:
			if entry.IsDir() {
				atomic.AddInt64(&scanStats.DirsProcessed, 1)
			} else {
				atomic.AddInt64(&scanStats.FilesProcessed, 1)
				atomic.AddInt64(&scanStats.TotalSize, info.Size())
			}
		case <-ctx.Done():
			return
		}

		// Recursively scan subdirectories
		if entry.IsDir() && !isSymlink(info) {
			wg.Add(1)
			go scanRecursive(ctx, fullPath, currentPath, wg, semaphore, resultChan, errChan)
		}
	}
}

// isSymlink checks if file is a symbolic link
func isSymlink(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}

// GetScanStats returns current scanning progress
func GetScanStats() ScanStats {
	scanMutex.RLock()
	defer scanMutex.RUnlock()
	return scanStats
}

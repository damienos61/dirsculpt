// main.go
// DirSculpt - Disk Space Analysis and Predictive Simulation Tool
// Ultra-lightweight CLI/TUI for low-resource systems
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	// Initialize application context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Runtime optimization for low-resource systems (Dual-Core)
	runtime.GOMAXPROCS(2)
	runtime.SetMutexProfileFraction(0)
	runtime.SetBlockProfileRate(0)

	// Determine root path to scan
	rootPath := "./"
	if len(os.Args) > 1 {
		rootPath = os.Args[1]
	}

	// Validate path
	stat, err := os.Stat(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: cannot access path '%s': %v\n", rootPath, err)
		os.Exit(1)
	}
	if !stat.IsDir() {
		fmt.Fprintf(os.Stderr, "❌ Error: '%s' is not a directory\n", rootPath)
		os.Exit(1)
	}

	// Normalize path
	rootPath, err = filepath.Abs(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: cannot resolve path: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🚀 DirSculpt starting...\n")
	fmt.Printf("📁 Scanning: %s\n\n", rootPath)

	// Initialize communication channels
	scanChan := make(chan ScanResult, 256)
	analyzerChan := make(chan AnalysisResult, 256)
	eventChan := make(chan KeyEvent, 64)
	errChan := make(chan error, 10)

	// Launch scanner goroutine
	go ScanDirectory(ctx, rootPath, scanChan, errChan)

	// Launch analyzer goroutine
	go AnalyzeFiles(ctx, scanChan, analyzerChan, errChan)

	// Initialize and run TUI
	err = RunTUI(ctx, rootPath, analyzerChan, eventChan, errChan)
	cancel()

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Fatal error: %v\n", err)
		os.Exit(1)
	}
}

// analyzer.go
// File type detection via magic numbers and compression simulation
// Virtual compression without disk writes
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AnalysisResult represents analyzed file metadata
type AnalysisResult struct {
	Path              string
	Name              string
	OriginalSize      int64
	SimulatedSize     int64
	CompressionRatio  float64
	MimeType          string
	CanCompress       bool
	IsRedundant       bool
	RedundancyReason  string
	IsMarkedForDelete bool
}

// GhostMode state for deletion simulation
type GhostMode struct {
	Enabled        bool
	MarkedPaths    map[string]bool
	ProjectedSpace int64
	mu             sync.RWMutex
}

var (
	ghostMode GhostMode
	cacheDirs = []string{
		".cache", "node_modules", "__pycache__", ".git", "venv",
		"vendor", "build", "dist", "target", ".gradle", ".m2",
		"tmp", "temp", ".temp", ".DS_Store", "Thumbs.db",
		".vs", ".vscode", ".idea",
	}
)

func init() {
	ghostMode.MarkedPaths = make(map[string]bool)
}

// AnalyzeFiles reads scan results and performs analysis
func AnalyzeFiles(
	ctx context.Context,
	scanChan <-chan ScanResult,
	analyzerChan chan<- AnalysisResult,
	errChan chan<- error,
) {
	defer close(analyzerChan)

	for {
		select {
		case result, ok := <-scanChan:
			if !ok {
				return
			}

			if result.IsDir {
				if isRedundantDir(result.Path) {
					analyzerChan <- AnalysisResult{
						Path:             result.Path,
						Name:             result.Name,
						OriginalSize:     result.Size,
						MimeType:         "directory/cache",
						IsRedundant:      true,
						RedundancyReason: "Known cache/build directory",
					}
				}
				continue
			}

			// Analyze file
			analysis := analyzeFile(result)
			select {
			case analyzerChan <- analysis:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// analyzeFile performs intelligent analysis on a single file
func analyzeFile(result ScanResult) AnalysisResult {
	analysis := AnalysisResult{
		Path:         result.Path,
		Name:         result.Name,
		OriginalSize: result.Size,
		MimeType:     "unknown",
	}

	// Detect MIME type
	mimeType := detectMimeType(result.Path)
	analysis.MimeType = mimeType

	// Determine compression potential
	switch {
	case isAlreadyCompressed(mimeType):
		analysis.CanCompress = false
		analysis.SimulatedSize = result.Size
		analysis.CompressionRatio = 0.0

	case isTextFile(mimeType):
		// DEFLATE compression: 75% reduction for text
		analysis.CanCompress = true
		analysis.SimulatedSize = int64(float64(result.Size) * 0.25)
		analysis.CompressionRatio = 0.75

	case isImageFile(mimeType):
		// Analyze image headers
		analysis.SimulatedSize = simulateImageCompression(result.Path, result.Size)
		analysis.CanCompress = analysis.SimulatedSize < result.Size
		if analysis.CanCompress {
			analysis.CompressionRatio = float64(result.Size-analysis.SimulatedSize) / float64(result.Size)
		}

	case isVideoFile(mimeType):
		// Video files rarely benefit from compression
		analysis.CanCompress = false
		analysis.SimulatedSize = result.Size
		analysis.CompressionRatio = 0.0

	default:
		// Generic compression
		analysis.CanCompress = true
		analysis.SimulatedSize = int64(float64(result.Size) * 0.5)
		analysis.CompressionRatio = 0.5
	}

	return analysis
}

// detectMimeType reads magic bytes for accurate type detection
func detectMimeType(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return "unknown"
	}
	defer file.Close()

	magicBytes := make([]byte, 8)
	n, _ := file.Read(magicBytes)
	if n == 0 {
		return "unknown/empty"
	}

	// Magic number signatures
	switch {
	case bytes.HasPrefix(magicBytes, []byte{0x50, 0x4B, 0x03, 0x04}):
		return "application/zip"
	case bytes.HasPrefix(magicBytes, []byte{0x89, 0x50, 0x4E, 0x47}):
		return "image/png"
	case bytes.HasPrefix(magicBytes, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(magicBytes, []byte{0x42, 0x4D}):
		return "image/bmp"
	case bytes.HasPrefix(magicBytes, []byte{0x47, 0x49, 0x46}):
		return "image/gif"
	case bytes.HasPrefix(magicBytes, []byte{0x1F, 0x8B}):
		return "application/gzip"
	case bytes.HasPrefix(magicBytes, []byte{0x42, 0x5A}):
		return "application/x-bzip2"
	case bytes.HasPrefix(magicBytes, []byte{0x25, 0x50, 0x44, 0x46}):
		return "application/pdf"
	case bytes.HasPrefix(magicBytes, []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}):
		return "video/mp4"
	case bytes.HasPrefix(magicBytes, []byte{0x49, 0x44, 0x33}) || bytes.HasPrefix(magicBytes, []byte{0xFF, 0xFB}):
		return "audio/mpeg"
	case isTextContent(magicBytes):
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// isTextContent checks for text file patterns
func isTextContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	textSignatures := [][]byte{
		{0x23, 0x21},      // Shebang
		{0x3C, 0x3F, 0x78}, // XML
		{0x7B},             // JSON
	}
	for _, sig := range textSignatures {
		if bytes.HasPrefix(data, sig) {
			return true
		}
	}
	printable := 0
	for _, b := range data {
		if (b >= 0x20 && b <= 0x7E) || b == '\n' || b == '\r' || b == '\t' {
			printable++
		}
	}
	return printable > len(data)/2
}

// isAlreadyCompressed checks file format
func isAlreadyCompressed(mimeType string) bool {
	compressed := []string{
		"application/zip", "application/gzip", "application/x-bzip2",
		"image/png", "image/jpeg", "image/gif", "video/mp4",
		"audio/mpeg", "application/x-7z-compressed",
	}
	for _, t := range compressed {
		if strings.Contains(mimeType, t) {
			return true
		}
	}
	return false
}

// isTextFile checks MIME type
func isTextFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") || 
		mimeType == "application/json" || 
		mimeType == "application/xml"
}

// isImageFile checks MIME type
func isImageFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// isVideoFile checks MIME type
func isVideoFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/") || 
		strings.HasPrefix(mimeType, "audio/")
}

// simulateImageCompression analyzes BMP headers
func simulateImageCompression(filePath string, originalSize int64) int64 {
	if !strings.HasSuffix(strings.ToLower(filePath), ".bmp") {
		return originalSize
	}

	file, err := os.Open(filePath)
	if err != nil {
		return originalSize
	}
	defer file.Close()

	header := make([]byte, 26)
	if _, err := file.Read(header); err != nil {
		return originalSize
	}

	if !bytes.Equal(header[0:2], []byte{0x42, 0x4D}) {
		return originalSize
	}

	width := int64(binary.LittleEndian.Uint32(header[18:22]))
	height := int64(binary.LittleEndian.Uint32(header[22:26]))

	pixelData := width * height * 3
	pngEstimate := int64(float64(pixelData) * 0.4)

	if pngEstimate < originalSize {
		return pngEstimate
	}
	return originalSize
}

// isRedundantDir checks for cache/build directories
func isRedundantDir(dirPath string) bool {
	name := filepath.Base(dirPath)
	for _, cacheDir := range cacheDirs {
		if name == cacheDir {
			return true
		}
	}
	return false
}

// ToggleGhostMode enables/disables ghost mode
func ToggleGhostMode(enable bool) {
	ghostMode.mu.Lock()
	defer ghostMode.mu.Unlock()
	ghostMode.Enabled = enable
	if !enable {
		ghostMode.MarkedPaths = make(map[string]bool)
		ghostMode.ProjectedSpace = 0
	}
}

// MarkForDeletion marks a file in ghost mode
func MarkForDeletion(path string, size int64, marked bool) {
	ghostMode.mu.Lock()
	defer ghostMode.mu.Unlock()

	if marked {
		ghostMode.MarkedPaths[path] = true
		ghostMode.ProjectedSpace += size
	} else {
		delete(ghostMode.MarkedPaths, path)
		ghostMode.ProjectedSpace -= size
	}
}

// GetGhostModeStats returns ghost mode statistics
func GetGhostModeStats() (bool, int64, int) {
	ghostMode.mu.RLock()
	defer ghostMode.mu.RUnlock()
	return ghostMode.Enabled, ghostMode.ProjectedSpace, len(ghostMode.MarkedPaths)
}

// IsMarkedForDeletion checks if path is marked
func IsMarkedForDeletion(path string) bool {
	ghostMode.mu.RLock()
	defer ghostMode.mu.RUnlock()
	return ghostMode.MarkedPaths[path]
}

// action_generator.go
// Generates executable scripts for operations
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// ActionScript represents a generated action script
type ActionScript struct {
	ScriptContent string
	ScriptType    string
	OutputPath    string
	Actions       []string
	TotalSavings  int64
}

// GenerateActionScript creates shell script from analyzed files
func GenerateActionScript(fileTree map[string]*FileNode, stats DiskStats, ghostModeEnabled bool) string {
	var script ActionScript
	script.Actions = make([]string, 0)

	if runtime.GOOS == "windows" {
		script.ScriptType = "powershell"
		script.generatePowerShellScript(fileTree, stats, ghostModeEnabled)
	} else {
		script.ScriptType = "bash"
		script.generateBashScript(fileTree, stats, ghostModeEnabled)
	}

	// Export to file
	_ = ExportActionScript(script)
	return script.ScriptContent
}

// generateBashScript creates bash script for Linux/macOS
func (as *ActionScript) generateBashScript(fileTree map[string]*FileNode, stats DiskStats, ghostModeEnabled bool) {
	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# DirSculpt Action Script - Auto-generated\n")
	sb.WriteString(fmt.Sprintf("# Generated: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("# WARNING: Review before execution!\n\n")

	sb.WriteString("set -e\n\n")

	sb.WriteString("echo '🔍 DirSculpt Action Script'\n")
	sb.WriteString(fmt.Sprintf("echo 'Projected savings: %s'\n", formatBytes(stats.CompressedSize)))
	sb.WriteString(fmt.Sprintf("echo 'Redundant to delete: %s'\n", formatBytes(stats.RedundantSize)))
	sb.WriteString("echo ''\n")

	sb.WriteString("read -p 'Continue? (y/N) ' -n 1 -r\n")
	sb.WriteString("echo\n")
	sb.WriteString("if [[ ! $REPLY =~ ^[Yy]$ ]]; then\n")
	sb.WriteString("  echo 'Cancelled'\n")
	sb.WriteString("  exit 0\n")
	sb.WriteString("fi\n\n")

	sb.WriteString("BACKUP_DIR=\"./dirsculpt_backup_$(date +%s)\"\n")
	sb.WriteString("mkdir -p \"$BACKUP_DIR\"\n")
	sb.WriteString("echo '📦 Backup: '$BACKUP_DIR'\n\n")

	// Process redundant directories
	sb.WriteString("# Remove redundant directories\n")
	for path := range fileTree {
		if isRedundantPath(path) {
			sb.WriteString(fmt.Sprintf("if [ -d '%s' ]; then\n", path))
			sb.WriteString(fmt.Sprintf("  echo 'Removing %s'\n", path))
			sb.WriteString(fmt.Sprintf("  rm -rf '%s'\n", path))
			sb.WriteString("fi\n\n")
			as.Actions = append(as.Actions, fmt.Sprintf("Remove: %s", path))
		}
	}

	// Ghost mode deletions
	if ghostModeEnabled {
		sb.WriteString("# Delete marked files\n")
		for path := range fileTree {
			if IsMarkedForDeletion(path) {
				sb.WriteString(fmt.Sprintf("if [ -f '%s' ]; then\n", path))
				sb.WriteString(fmt.Sprintf("  echo 'Deleting %s'\n", path))
				sb.WriteString(fmt.Sprintf("  rm '%s'\n", path))
				sb.WriteString("fi\n\n")
				as.Actions = append(as.Actions, fmt.Sprintf("Delete: %s", path))
			}
		}
	}

	sb.WriteString("echo '✅ Completed'\n")
	sb.WriteString("echo 'Backup: '$BACKUP_DIR'\n")

	as.ScriptContent = sb.String()
}

// generatePowerShellScript creates PowerShell script for Windows
func (as *ActionScript) generatePowerShellScript(fileTree map[string]*FileNode, stats DiskStats, ghostModeEnabled bool) {
	var sb strings.Builder

	sb.WriteString("# DirSculpt Action Script - Windows\n")
	sb.WriteString(fmt.Sprintf("# Generated: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("# WARNING: Review before execution!\n\n")

	sb.WriteString("$ErrorActionPreference = 'Stop'\n\n")

	sb.WriteString("Write-Host '🔍 DirSculpt Action Script' -ForegroundColor Cyan\n")
	sb.WriteString(fmt.Sprintf("Write-Host 'Projected savings: %s'\n", formatBytes(stats.CompressedSize)))
	sb.WriteString(fmt.Sprintf("Write-Host 'Redundant to delete: %s'\n", formatBytes(stats.RedundantSize)))

	sb.WriteString("$response = Read-Host 'Continue? (y/N)'\n")
	sb.WriteString("if ($response -ne 'y' -and $response -ne 'Y') {\n")
	sb.WriteString("  Write-Host 'Cancelled'\n")
	sb.WriteString("  exit 0\n")
	sb.WriteString("}\n\n")

	sb.WriteString("$BackupDir = \"./dirsculpt_backup_$(Get-Date -Format 'yyyyMMdd_HHmmss')\"\n")
	sb.WriteString("New-Item -ItemType Directory -Path $BackupDir -Force | Out-Null\n")
	sb.WriteString("Write-Host \"📦 Backup: $BackupDir\" -ForegroundColor Green\n\n")

	// Process redundant directories
	sb.WriteString("# Remove redundant directories\n")
	for path := range fileTree {
		if isRedundantPath(path) {
			sb.WriteString(fmt.Sprintf("if (Test-Path '%s') {\n", path))
			sb.WriteString(fmt.Sprintf("  Write-Host 'Removing %s'\n", path))
			sb.WriteString(fmt.Sprintf("  Remove-Item -Path '%s' -Recurse -Force\n", path))
			sb.WriteString("}\n\n")
			as.Actions = append(as.Actions, fmt.Sprintf("Remove: %s", path))
		}
	}

	// Ghost mode deletions
	if ghostModeEnabled {
		sb.WriteString("# Delete marked files\n")
		for path := range fileTree {
			if IsMarkedForDeletion(path) {
				sb.WriteString(fmt.Sprintf("if (Test-Path '%s') {\n", path))
				sb.WriteString(fmt.Sprintf("  Write-Host 'Deleting %s'\n", path))
				sb.WriteString(fmt.Sprintf("  Remove-Item -Path '%s' -Force\n", path))
				sb.WriteString("}\n\n")
				as.Actions = append(as.Actions, fmt.Sprintf("Delete: %s", path))
			}
		}
	}

	sb.WriteString("Write-Host '✅ Completed' -ForegroundColor Green\n")
	sb.WriteString("Write-Host \"Backup: $BackupDir\" -ForegroundColor Green\n")

	as.ScriptContent = sb.String()
}

// isRedundantPath checks if path matches redundant patterns
func isRedundantPath(path string) bool {
	redundantPatterns := []string{
		".cache", "node_modules", "__pycache__", ".git", "venv",
		"vendor", "build", "dist", "target", ".gradle",
	}
	for _, pattern := range redundantPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// ExportActionScript saves script to file
func ExportActionScript(script ActionScript) error {
	timestamp := time.Now().Format("20060102_150405")
	var filename string

	if runtime.GOOS == "windows" {
		filename = fmt.Sprintf("dirsculpt_actions_%s.ps1", timestamp)
	} else {
		filename = fmt.Sprintf("dirsculpt_actions_%s.sh", timestamp)
	}

	err := os.WriteFile(filename, []byte(script.ScriptContent), 0755)
	if err != nil {
		return fmt.Errorf("failed to write script: %w", err)
	}

	fmt.Printf("✅ Script exported: %s\n", filename)
	return nil
}

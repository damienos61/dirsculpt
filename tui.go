// tui.go
// Terminal UI rendering with Bubbletea
// Optimized for 60 FPS responsiveness
package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents TUI application state
type Model struct {
	ctx          context.Context
	rootPath     string
	analyzer     <-chan AnalysisResult
	events       <-chan KeyEvent
	err          <-chan error
	fileTree     map[string]*FileNode
	selectedPath string
	scrollOffset int
	width        int
	height       int
	ghostMode    bool
	compressMode bool
	stats        DiskStats
	actionLog    []string
	mu           sync.RWMutex
}

// FileNode represents file/directory in tree
type FileNode struct {
	Path              string
	Name              string
	Size              int64
	SimulatedSize     int64
	Children          map[string]*FileNode
	IsDir             bool
	IsRedundant       bool
	RedundancyReason  string
	CanCompress       bool
	CompressionRatio  float64
	MimeType          string
	IsMarkedForDelete bool
}

// DiskStats aggregates space information
type DiskStats struct {
	TotalSize       int64
	CompressedSize  int64
	RedundantSize   int64
	ProjectedDelete int64
}

// Message types
type analysisMsg struct{ result AnalysisResult }
type keyEventMsg struct{ event KeyEvent }
type errorMsg struct{ err error }
type actionScriptMsg struct{ script string }

// InitialModel creates new TUI model
func InitialModel(ctx context.Context, rootPath string, analyzer <-chan AnalysisResult, events <-chan KeyEvent, err <-chan error) Model {
	return Model{
		ctx:      ctx,
		rootPath: rootPath,
		analyzer: analyzer,
		events:   events,
		err:      err,
		fileTree: make(map[string]*FileNode),
		stats:    DiskStats{},
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenAnalyzer(),
		m.listenEvents(),
		m.listenErrors(),
	)
}

// Update processes messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "g":
			m.ghostMode = !m.ghostMode
			ToggleGhostMode(m.ghostMode)
			return m, nil
		case "z":
			m.compressMode = !m.compressMode
			return m, nil
		case "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		case "down":
			m.scrollOffset++
			return m, nil
		case "ctrl+x":
			return m, m.generateActionScript()
		}

	case analysisMsg:
		m.updateTreeWithAnalysis(msg.result)
		return m, m.listenAnalyzer()

	case tea.QuitMsg:
		return m, tea.Quit
	}

	return m, nil
}

// View renders the TUI
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing...\n"
	}

	var sb strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Render("📊 DirSculpt - Disk Space Analyzer")

	sb.WriteString(header)
	sb.WriteString("\n")

	// Status line
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	ghost := "OFF"
	if m.ghostMode {
		ghost = "ON"
	}
	compress := "OFF"
	if m.compressMode {
		compress = "ON"
	}
	status := fmt.Sprintf("Path: %s | Ghost Mode: %s | Compression: %s", 
		truncatePath(m.rootPath, 40), ghost, compress)
	sb.WriteString(statusStyle.Render(status))
	sb.WriteString("\n\n")

	// Stats display
	stats := m.renderStats()
	sb.WriteString(stats)
	sb.WriteString("\n\n")

	// Tree view
	tree := m.renderTree()
	sb.WriteString(tree)
	sb.WriteString("\n\n")

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	help := helpStyle.Render("↑↓: Navigate | g: Ghost Mode | z: Compression | Ctrl+X: Script | Esc: Quit")
	sb.WriteString(help)

	return sb.String()
}

// renderStats generates statistics display
func (m Model) renderStats() string {
	statsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Foreground(lipgloss.Color("10"))

	total := formatBytes(m.stats.TotalSize)
	compressed := formatBytes(m.stats.CompressedSize)
	redundant := formatBytes(m.stats.RedundantSize)
	projected := formatBytes(m.stats.ProjectedDelete)

	content := fmt.Sprintf(
		"Total: %s | Compressible: %s | Redundant: %s | Projected: %s",
		total, compressed, redundant, projected,
	)

	return statsStyle.Render(content)
}

// renderTree generates directory tree view
func (m Model) renderTree() string {
	var sb strings.Builder

	lines := m.buildTreeLines()
	maxLines := m.height - 15

	for i := m.scrollOffset; i < len(lines) && i < m.scrollOffset+maxLines; i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildTreeLines creates tree visualization
func (m Model) buildTreeLines() []string {
	var lines []string

	for _, node := range m.fileTree {
		lines = append(lines, m.nodeToLine(node, 0)...)
	}

	return lines
}

// nodeToLine converts node to display lines
func (m Model) nodeToLine(node *FileNode, depth int) []string {
	var lines []string

	prefix := strings.Repeat("  ", depth)
	var style lipgloss.Style

	if node.IsRedundant {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
	} else if node.IsMarkedForDelete && m.ghostMode {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Strikethrough(true)
	} else if node.CanCompress && m.compressMode {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
	}

	icon := "📄"
	if node.IsDir {
		icon = "📁"
	}

	sizeStr := formatBytes(node.Size)
	if m.compressMode && node.CanCompress {
		compStr := formatBytes(node.SimulatedSize)
		ratio := int(node.CompressionRatio * 100)
		sizeStr = fmt.Sprintf("%s → %s (%d%%)", sizeStr, compStr, ratio)
	}

	line := fmt.Sprintf("%s%s %s (%s)", prefix, icon, node.Name, sizeStr)
	if node.IsRedundant {
		line += fmt.Sprintf(" ⚠️ [%s]", node.RedundancyReason)
	}

	lines = append(lines, style.Render(line))

	// Add children
	if node.IsDir && len(node.Children) > 0 {
		childNames := make([]string, 0, len(node.Children))
		for name := range node.Children {
			childNames = append(childNames, name)
		}
		sort.Strings(childNames)

		for _, name := range childNames {
			lines = append(lines, m.nodeToLine(node.Children[name], depth+1)...)
		}
	}

	return lines
}

// updateTreeWithAnalysis adds analysis result to tree
func (m *Model) updateTreeWithAnalysis(result AnalysisResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node := &FileNode{
		Path:             result.Path,
		Name:             result.Name,
		Size:             result.OriginalSize,
		SimulatedSize:    result.SimulatedSize,
		IsDir:            false,
		CanCompress:      result.CanCompress,
		CompressionRatio: result.CompressionRatio,
		MimeType:         result.MimeType,
		IsRedundant:      result.IsRedundant,
		RedundancyReason: result.RedundancyReason,
		Children:         make(map[string]*FileNode),
	}

	m.fileTree[result.Path] = node

	m.stats.TotalSize += result.OriginalSize
	if result.CanCompress {
		m.stats.CompressedSize += result.SimulatedSize
	}
	if result.IsRedundant {
		m.stats.RedundantSize += result.OriginalSize
	}
}

// listenAnalyzer creates command to listen for analysis results
func (m Model) listenAnalyzer() tea.Cmd {
	return func() tea.Msg {
		select {
		case result, ok := <-m.analyzer:
			if !ok {
				return nil
			}
			return analysisMsg{result}
		case <-m.ctx.Done():
			return tea.QuitMsg{}
		}
	}
}

// listenEvents creates command to listen for key events
func (m Model) listenEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-m.events:
			if !ok {
				return nil
			}
			return keyEventMsg{event}
		case <-m.ctx.Done():
			return tea.QuitMsg{}
		}
	}
}

// listenErrors creates command to listen for errors
func (m Model) listenErrors() tea.Cmd {
	return func() tea.Msg {
		select {
		case err, ok := <-m.err:
			if !ok {
				return nil
			}
			return errorMsg{err}
		case <-m.ctx.Done():
			return tea.QuitMsg{}
		}
	}
}

// generateActionScript creates bash/powershell script
func (m Model) generateActionScript() tea.Cmd {
	return func() tea.Msg {
		script := GenerateActionScript(m.fileTree, m.stats, m.ghostMode)
		return actionScriptMsg{script}
	}
}

// RunTUI starts the terminal UI
func RunTUI(ctx context.Context, rootPath string, analyzer <-chan AnalysisResult, events <-chan KeyEvent, errChan <-chan error) error {
	model := InitialModel(ctx, rootPath, analyzer, events, errChan)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// formatBytes converts bytes to human-readable format
func formatBytes(b int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(b)
	for _, unit := range units {
		if size < 1024 {
			if size < 0 {
				size = 0
			}
			return fmt.Sprintf("%.1f%s", size, unit)
		}
		size /= 1024
	}
	return fmt.Sprintf("%.1f%s", size, units[len(units)-1])
}

// truncatePath shortens path for display
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

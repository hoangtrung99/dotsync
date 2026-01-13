package components

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dotsync/internal/ui"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FilePreview displays file content with syntax highlighting using viewport
type FilePreview struct {
	viewport    viewport.Model
	highlighter *ui.Highlighter

	// File info
	FilePath   string
	FileName   string
	FileSize   int64
	TotalLines int

	// Dimensions
	Width  int
	Height int

	// State
	ready bool

	// Styles
	lineNumStyle lipgloss.Style
	headerStyle  lipgloss.Style
	infoStyle    lipgloss.Style
	borderStyle  lipgloss.Style
}

// NewFilePreview creates a new FilePreview with viewport
func NewFilePreview() *FilePreview {
	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	return &FilePreview{
		viewport:    vp,
		highlighter: ui.NewHighlighter(),
		Width:       80,
		Height:      20,
		lineNumStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086")).
			Width(5).
			Align(lipgloss.Right),
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#89b4fa")),
		infoStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086")),
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#89b4fa")).
			Padding(0, 1),
	}
}

// SetSize updates the viewport dimensions
func (p *FilePreview) SetSize(width, height int) {
	p.Width = width
	p.Height = height

	// Account for header (3 lines) and border (2 lines)
	contentHeight := height - 5
	if contentHeight < 5 {
		contentHeight = 5
	}
	contentWidth := width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	p.viewport.Width = contentWidth
	p.viewport.Height = contentHeight
	p.ready = true
}

// Load loads a file for preview
func (p *FilePreview) Load(path string) error {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return p.loadDirectory(path)
	}

	// Check file size - don't load huge files
	if info.Size() > 1024*1024 { // 1MB limit
		p.setMessage(path, info.Size(), []string{
			"",
			"  ⚠️  File is too large to preview",
			fmt.Sprintf("  Size: %s", formatBytes(info.Size())),
			"",
			"  Use an external editor to view this file.",
		})
		return nil
	}

	// Read file content
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Check if binary
	if isBinaryContent(data) {
		p.setMessage(path, info.Size(), []string{
			"",
			"  ⚠️  Binary file - cannot preview",
			fmt.Sprintf("  Size: %s", formatBytes(info.Size())),
			"",
			"  Use an external editor to view this file.",
		})
		return nil
	}

	// Split into lines
	content := string(data)
	lines := strings.Split(content, "\n")

	// Build content with line numbers and syntax highlighting
	var b strings.Builder
	for i, line := range lines {
		// Line number
		lineNum := p.lineNumStyle.Render(fmt.Sprintf("%d", i+1))

		// Syntax highlighted line
		highlighted := p.highlighter.HighlightLine(line, path)

		// Truncate very long lines for display
		maxWidth := p.viewport.Width - 10
		if maxWidth < 40 {
			maxWidth = 40
		}

		// Use visible length for truncation (accounting for ANSI codes)
		visibleLine := stripAnsi(highlighted)
		if len(visibleLine) > maxWidth {
			// Truncate the original line and re-highlight
			truncated := line
			if len(line) > maxWidth-3 {
				truncated = line[:maxWidth-3] + "..."
			}
			highlighted = p.highlighter.HighlightLine(truncated, path)
		}

		b.WriteString(lineNum + " │ " + highlighted)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}

	p.FilePath = path
	p.FileName = filepath.Base(path)
	p.FileSize = info.Size()
	p.TotalLines = len(lines)
	p.viewport.SetContent(b.String())
	p.viewport.GotoTop()

	return nil
}

// loadDirectory shows directory contents
func (p *FilePreview) loadDirectory(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  📁 Directory: %s\n", filepath.Base(path)))
	b.WriteString(fmt.Sprintf("  %d items\n", len(entries)))
	b.WriteString("\n")
	b.WriteString("  Contents:\n")
	b.WriteString("  " + strings.Repeat("─", 40) + "\n")

	for _, entry := range entries {
		icon := "📄"
		if entry.IsDir() {
			icon = "📁"
		}
		info, _ := entry.Info()
		size := ""
		if info != nil && !entry.IsDir() {
			size = formatBytes(info.Size())
		}
		b.WriteString(fmt.Sprintf("  %s %s  %s\n", icon, entry.Name(), size))
	}

	p.FilePath = path
	p.FileName = filepath.Base(path)
	p.FileSize = 0
	p.TotalLines = len(entries) + 6
	p.viewport.SetContent(b.String())
	p.viewport.GotoTop()

	return nil
}

// setMessage sets a simple message content
func (p *FilePreview) setMessage(path string, size int64, lines []string) {
	p.FilePath = path
	p.FileName = filepath.Base(path)
	p.FileSize = size
	p.TotalLines = len(lines)
	p.viewport.SetContent(strings.Join(lines, "\n"))
	p.viewport.GotoTop()
}

// Update handles messages for viewport scrolling
func (p *FilePreview) Update(msg tea.Msg) (*FilePreview, tea.Cmd) {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

// View renders the preview
func (p *FilePreview) View() string {
	var b strings.Builder

	// Header
	header := p.headerStyle.Render(fmt.Sprintf("📄 %s", p.FileName))
	sizeInfo := p.infoStyle.Render(fmt.Sprintf("  %s  %d lines", formatBytes(p.FileSize), p.TotalLines))
	b.WriteString(header + sizeInfo + "\n")

	// File path
	b.WriteString(p.infoStyle.Render(p.FilePath) + "\n")

	// Separator
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#313244")).
		Render(strings.Repeat("─", p.Width-4)) + "\n")

	// Viewport content
	b.WriteString(p.viewport.View())

	// Scroll indicator
	if p.TotalLines > p.viewport.Height {
		scrollPercent := p.viewport.ScrollPercent() * 100
		scrollInfo := fmt.Sprintf("─── %.0f%% ───", scrollPercent)
		b.WriteString("\n" + p.infoStyle.Render(scrollInfo))
	}

	// Wrap in border
	style := p.borderStyle.
		Width(p.Width).
		Height(p.Height)

	return style.Render(b.String())
}

// ScrollUp scrolls up (for backward compatibility)
func (p *FilePreview) ScrollUp() {
	p.viewport.LineUp(1)
}

// ScrollDown scrolls down (for backward compatibility)
func (p *FilePreview) ScrollDown() {
	p.viewport.LineDown(1)
}

// PageUp scrolls up by a page (for backward compatibility)
func (p *FilePreview) PageUp() {
	p.viewport.ViewUp()
}

// PageDown scrolls down by a page (for backward compatibility)
func (p *FilePreview) PageDown() {
	p.viewport.ViewDown()
}

// GoToTop goes to the beginning (for backward compatibility)
func (p *FilePreview) GoToTop() {
	p.viewport.GotoTop()
}

// GoToBottom goes to the end (for backward compatibility)
func (p *FilePreview) GoToBottom() {
	p.viewport.GotoBottom()
}

// isBinaryContent checks if content appears to be binary
func isBinaryContent(data []byte) bool {
	// Check first 512 bytes for null bytes or high proportion of non-printable chars
	checkLen := 512
	if len(data) < checkLen {
		checkLen = len(data)
	}

	nonPrintable := 0
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return true // Null byte = binary
		}
		if data[i] < 32 && data[i] != '\n' && data[i] != '\r' && data[i] != '\t' {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider binary
	return float64(nonPrintable)/float64(checkLen) > 0.3
}

// formatBytes formats bytes to human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// stripAnsi removes ANSI escape codes from a string
func stripAnsi(str string) string {
	var result strings.Builder
	inEscape := false

	for i := 0; i < len(str); i++ {
		if str[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if str[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(str[i])
	}

	return result.String()
}

package components

import (
	"fmt"
	"strings"

	"dotsync/internal/sync"
	"dotsync/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// DiffView displays a side-by-side diff of two files
type DiffView struct {
	Width  int
	Height int

	FilePath    string
	LocalPath   string
	DotfilePath string
	DiffResult  *sync.DiffResult

	// Navigation
	ScrollOffset int
	CurrentHunk  int

	// Syntax highlighting
	highlighter     *ui.Highlighter
	enableHighlight bool

	// Styles
	addStyle     lipgloss.Style
	deleteStyle  lipgloss.Style
	contextStyle lipgloss.Style
	headerStyle  lipgloss.Style
}

// NewDiffView creates a new DiffView
func NewDiffView() *DiffView {
	return &DiffView{
		Width:           80,
		Height:          20,
		highlighter:     ui.NewHighlighter(),
		enableHighlight: true,
		addStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e3a1")),
		deleteStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8")),
		contextStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086")),
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#89b4fa")),
	}
}

// SetDiff sets the diff result to display
func (d *DiffView) SetDiff(result *sync.DiffResult, localPath, dotfilePath string) {
	d.DiffResult = result
	d.LocalPath = localPath
	d.DotfilePath = dotfilePath
	d.ScrollOffset = 0
	d.CurrentHunk = 0
}

// ScrollUp scrolls the view up
func (d *DiffView) ScrollUp() {
	if d.ScrollOffset > 0 {
		d.ScrollOffset--
	}
}

// ScrollDown scrolls the view down
func (d *DiffView) ScrollDown() {
	d.ScrollOffset++
}

// NextHunk moves to the next hunk
func (d *DiffView) NextHunk() {
	if d.DiffResult != nil && d.CurrentHunk < len(d.DiffResult.Hunks)-1 {
		d.CurrentHunk++
	}
}

// PrevHunk moves to the previous hunk
func (d *DiffView) PrevHunk() {
	if d.CurrentHunk > 0 {
		d.CurrentHunk--
	}
}

// View renders the diff view
func (d *DiffView) View() string {
	if d.DiffResult == nil {
		return "No diff to display"
	}

	var b strings.Builder

	// Header
	header := d.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Stats
	stats := d.renderStats()
	b.WriteString(stats)
	b.WriteString("\n\n")

	// Diff content
	content := d.renderDiff()
	b.WriteString(content)

	// Footer
	b.WriteString("\n")
	b.WriteString(d.renderFooter())

	return b.String()
}

func (d *DiffView) renderHeader() string {
	title := d.headerStyle.Render("📊 Diff View")

	var fileName string
	if d.DiffResult.OldPath != "" {
		fileName = d.DiffResult.OldPath
	} else {
		fileName = d.DiffResult.NewPath
	}

	fileType := ui.GetFileType(fileName)
	highlightStatus := ""
	if d.enableHighlight {
		highlightStatus = " [syntax on]"
	}

	return fmt.Sprintf("%s  %s  %s%s", title, ui.MutedStyle.Render(fileName),
		ui.SyncedStyle.Render(fileType), ui.MutedStyle.Render(highlightStatus))
}

// ToggleHighlight toggles syntax highlighting
func (d *DiffView) ToggleHighlight() {
	d.enableHighlight = !d.enableHighlight
}

func (d *DiffView) renderStats() string {
	if d.DiffResult.Identical {
		return ui.SyncedStyle.Render("✓ Files are identical")
	}

	var parts []string
	if d.DiffResult.LinesAdded > 0 {
		parts = append(parts, d.addStyle.Render(fmt.Sprintf("+%d", d.DiffResult.LinesAdded)))
	}
	if d.DiffResult.LinesRemoved > 0 {
		parts = append(parts, d.deleteStyle.Render(fmt.Sprintf("-%d", d.DiffResult.LinesRemoved)))
	}

	hunks := fmt.Sprintf("%d hunks", len(d.DiffResult.Hunks))
	return strings.Join(parts, " ") + "  " + ui.MutedStyle.Render(hunks)
}

func (d *DiffView) renderDiff() string {
	if d.DiffResult.Identical {
		return ui.MutedStyle.Render("No differences found")
	}

	var lines []string
	lineWidth := d.Width - 4 // Padding

	for hunkIdx, hunk := range d.DiffResult.Hunks {
		// Hunk header
		hunkHeader := fmt.Sprintf("@@ Hunk %d @@", hunkIdx+1)
		if hunkIdx == d.CurrentHunk {
			hunkHeader = ui.SelectedItemStyle.Render(hunkHeader)
		} else {
			hunkHeader = ui.MutedStyle.Render(hunkHeader)
		}
		lines = append(lines, hunkHeader)

		// Diff lines
		for _, diffLine := range hunk.DiffLines {
			line := d.formatDiffLine(diffLine, lineWidth)
			lines = append(lines, line)
		}

		lines = append(lines, "") // Blank line between hunks
	}

	// Apply scroll offset
	visibleLines := d.Height - 8 // Reserve space for header/footer
	if visibleLines < 1 {
		visibleLines = 10
	}

	start := d.ScrollOffset
	if start >= len(lines) {
		start = 0
	}
	end := start + visibleLines
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

func (d *DiffView) formatDiffLine(line sync.DiffLine, maxWidth int) string {
	content := line.Content
	if len(content) > maxWidth-2 {
		content = content[:maxWidth-5] + "..."
	}

	// Get filename for syntax highlighting
	var fileName string
	if d.DiffResult.OldPath != "" {
		fileName = d.DiffResult.OldPath
	} else {
		fileName = d.DiffResult.NewPath
	}

	// Apply syntax highlighting to context lines if enabled
	if d.enableHighlight && line.Type == sync.DiffEqual && d.highlighter != nil {
		content = d.highlighter.HighlightLine(content, fileName)
	}

	switch line.Type {
	case sync.DiffInsert:
		return d.addStyle.Render("+ " + content)
	case sync.DiffDelete:
		return d.deleteStyle.Render("- " + content)
	default:
		return d.contextStyle.Render("  ") + content
	}
}

func (d *DiffView) renderFooter() string {
	items := []string{
		ui.RenderHelpItem("j/k", "scroll"),
		ui.RenderHelpItem("n/N", "next/prev hunk"),
		ui.RenderHelpItem("1", "keep local"),
		ui.RenderHelpItem("2", "use dotfiles"),
		ui.RenderHelpItem("m", "merge"),
		ui.RenderHelpItem("h", "highlight"),
		ui.RenderHelpItem("ESC", "close"),
	}
	return ui.HelpBarStyle.Render(strings.Join(items, "  "))
}

// HasChanges returns true if there are differences
func (d *DiffView) HasChanges() bool {
	return d.DiffResult != nil && !d.DiffResult.Identical
}

// HunkCount returns the number of hunks
func (d *DiffView) HunkCount() int {
	if d.DiffResult == nil {
		return 0
	}
	return len(d.DiffResult.Hunks)
}

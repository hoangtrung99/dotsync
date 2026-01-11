package components

import (
	"fmt"
	"strings"

	"dotsync/internal/sync"
	"dotsync/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// MergeView displays merge UI for conflict resolution
type MergeView struct {
	Width  int
	Height int

	MergeResult  *sync.MergeResult
	CurrentHunk  int
	ScrollOffset int

	// Styles
	localStyle    lipgloss.Style
	dotfilesStyle lipgloss.Style
	contextStyle  lipgloss.Style
	headerStyle   lipgloss.Style
	resolvedStyle lipgloss.Style
}

// NewMergeView creates a new MergeView
func NewMergeView() *MergeView {
	return &MergeView{
		Width:  80,
		Height: 20,
		localStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8")), // Red for local/delete
		dotfilesStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e3a1")), // Green for dotfiles/add
		contextStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086")), // Gray for context
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#89b4fa")), // Blue for headers
		resolvedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e3a1")).
			Bold(true),
	}
}

// SetMerge sets the merge result to display
func (m *MergeView) SetMerge(result *sync.MergeResult) {
	m.MergeResult = result
	m.CurrentHunk = 0
	m.ScrollOffset = 0
}

// NextHunk moves to the next hunk
func (m *MergeView) NextHunk() {
	if m.MergeResult != nil && m.CurrentHunk < len(m.MergeResult.Hunks)-1 {
		m.CurrentHunk++
		m.ScrollOffset = 0
	}
}

// PrevHunk moves to the previous hunk
func (m *MergeView) PrevHunk() {
	if m.CurrentHunk > 0 {
		m.CurrentHunk--
		m.ScrollOffset = 0
	}
}

// ScrollUp scrolls the view up
func (m *MergeView) ScrollUp() {
	if m.ScrollOffset > 0 {
		m.ScrollOffset--
	}
}

// ScrollDown scrolls the view down
func (m *MergeView) ScrollDown() {
	m.ScrollOffset++
}

// ResolveCurrentKeepLocal resolves current hunk by keeping local
func (m *MergeView) ResolveCurrentKeepLocal() bool {
	if m.MergeResult != nil && m.CurrentHunk < len(m.MergeResult.Hunks) {
		m.MergeResult.ResolveHunk(m.CurrentHunk, sync.ResolutionKeepLocal)
		// Auto-advance to next unresolved hunk
		m.advanceToNextUnresolved()
		return true
	}
	return false
}

// ResolveCurrentUseDotfiles resolves current hunk by using dotfiles version
func (m *MergeView) ResolveCurrentUseDotfiles() bool {
	if m.MergeResult != nil && m.CurrentHunk < len(m.MergeResult.Hunks) {
		m.MergeResult.ResolveHunk(m.CurrentHunk, sync.ResolutionUseDotfiles)
		// Auto-advance to next unresolved hunk
		m.advanceToNextUnresolved()
		return true
	}
	return false
}

// advanceToNextUnresolved moves to the next unresolved hunk
func (m *MergeView) advanceToNextUnresolved() {
	if m.MergeResult == nil {
		return
	}

	// Look for next unresolved hunk starting from current position
	for i := m.CurrentHunk + 1; i < len(m.MergeResult.Hunks); i++ {
		if m.MergeResult.Hunks[i].Resolution == sync.ResolutionPending {
			m.CurrentHunk = i
			m.ScrollOffset = 0
			return
		}
	}

	// Wrap around to beginning
	for i := 0; i < m.CurrentHunk; i++ {
		if m.MergeResult.Hunks[i].Resolution == sync.ResolutionPending {
			m.CurrentHunk = i
			m.ScrollOffset = 0
			return
		}
	}
}

// IsFullyResolved returns true if all hunks are resolved
func (m *MergeView) IsFullyResolved() bool {
	return m.MergeResult != nil && m.MergeResult.IsFullyResolved
}

// View renders the merge view
func (m *MergeView) View() string {
	if m.MergeResult == nil {
		return "No merge in progress"
	}

	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Progress bar
	progress := m.renderProgress()
	b.WriteString(progress)
	b.WriteString("\n\n")

	// Current hunk
	hunkView := m.renderCurrentHunk()
	b.WriteString(hunkView)

	// Footer
	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m *MergeView) renderHeader() string {
	title := m.headerStyle.Render("🔀 Merge View")

	fileName := m.MergeResult.FilePath
	if fileName == "" {
		fileName = m.MergeResult.LocalPath
	}

	hunkInfo := fmt.Sprintf("[%d/%d hunks]", m.CurrentHunk+1, m.MergeResult.TotalHunks)

	return fmt.Sprintf("%s  %s  %s",
		title,
		ui.MutedStyle.Render(fileName),
		ui.MutedStyle.Render(hunkInfo),
	)
}

func (m *MergeView) renderProgress() string {
	resolved := m.MergeResult.ResolvedHunks
	total := m.MergeResult.TotalHunks

	// Build progress bar
	barWidth := 30
	filledWidth := 0
	if total > 0 {
		filledWidth = (resolved * barWidth) / total
	}

	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)

	statusText := fmt.Sprintf("%d/%d resolved", resolved, total)
	if m.MergeResult.IsFullyResolved {
		statusText = m.resolvedStyle.Render("✓ All conflicts resolved!")
	}

	return fmt.Sprintf("[%s] %s", bar, statusText)
}

func (m *MergeView) renderCurrentHunk() string {
	if m.CurrentHunk >= len(m.MergeResult.Hunks) {
		return "No hunks to display"
	}

	hunk := m.MergeResult.Hunks[m.CurrentHunk]
	var lines []string

	// Hunk header with resolution status
	status := ""
	switch hunk.Resolution {
	case sync.ResolutionPending:
		status = ui.MutedStyle.Render("(pending)")
	case sync.ResolutionKeepLocal:
		status = m.localStyle.Render("✓ Keep Local")
	case sync.ResolutionUseDotfiles:
		status = m.dotfilesStyle.Render("✓ Use Dotfiles")
	case sync.ResolutionManual:
		status = m.headerStyle.Render("✓ Manual")
	}

	hunkHeader := fmt.Sprintf("═══ Conflict #%d %s ═══", hunk.Index+1, status)
	lines = append(lines, m.headerStyle.Render(hunkHeader))
	lines = append(lines, "")

	// Context before (if any)
	for _, line := range hunk.ContextBefore {
		lines = append(lines, m.contextStyle.Render("  "+line))
	}

	// Conflict markers and content
	lines = append(lines, m.localStyle.Render("<<<<<<< LOCAL"))
	for _, line := range hunk.LocalLines {
		lines = append(lines, m.localStyle.Render("- "+line))
	}

	lines = append(lines, m.contextStyle.Render("======="))

	for _, line := range hunk.DotfilesLines {
		lines = append(lines, m.dotfilesStyle.Render("+ "+line))
	}
	lines = append(lines, m.dotfilesStyle.Render(">>>>>>> DOTFILES"))

	// Context after (if any)
	for _, line := range hunk.ContextAfter {
		lines = append(lines, m.contextStyle.Render("  "+line))
	}

	// Apply scroll and limit
	visibleLines := m.Height - 12
	if visibleLines < 5 {
		visibleLines = 10
	}

	start := m.ScrollOffset
	if start >= len(lines) {
		start = 0
	}
	end := start + visibleLines
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

func (m *MergeView) renderFooter() string {
	items := []string{
		ui.RenderHelpItem("j/k", "scroll"),
		ui.RenderHelpItem("n/N", "next/prev hunk"),
		ui.RenderHelpItem("1", "keep local"),
		ui.RenderHelpItem("2", "use dotfiles"),
	}

	if m.IsFullyResolved() {
		items = append(items, ui.RenderHelpItem("ENTER", "save merge"))
	}

	items = append(items, ui.RenderHelpItem("ESC", "cancel"))

	return ui.HelpBarStyle.Render(strings.Join(items, "  "))
}

// HunkCount returns the number of hunks
func (m *MergeView) HunkCount() int {
	if m.MergeResult == nil {
		return 0
	}
	return len(m.MergeResult.Hunks)
}

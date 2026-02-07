package components

import (
	"fmt"
	"strings"

	"dotsync/internal/suggestions"
	"dotsync/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// SuggestionBar displays smart suggestions at the top of the screen
type SuggestionBar struct {
	Suggestion *suggestions.Suggestion
	Width      int
	Visible    bool
}

// NewSuggestionBar creates a new suggestion bar
func NewSuggestionBar() *SuggestionBar {
	return &SuggestionBar{
		Suggestion: nil,
		Width:      80,
		Visible:    true,
	}
}

// SetSuggestion updates the current suggestion
func (s *SuggestionBar) SetSuggestion(suggestion *suggestions.Suggestion) {
	s.Suggestion = suggestion
}

// SetWidth sets the width of the bar
func (s *SuggestionBar) SetWidth(width int) {
	s.Width = width
}

// Show shows the suggestion bar
func (s *SuggestionBar) Show() {
	s.Visible = true
}

// Hide hides the suggestion bar
func (s *SuggestionBar) Hide() {
	s.Visible = false
}

// IsVisible returns whether the bar is visible
func (s *SuggestionBar) IsVisible() bool {
	return s.Visible && s.Suggestion != nil && !s.Suggestion.IsEmpty()
}

// View renders the suggestion bar
func (s *SuggestionBar) View() string {
	if !s.IsVisible() {
		return ""
	}

	var b strings.Builder

	// Style based on suggestion type
	var borderColor lipgloss.Color
	var icon string

	switch s.Suggestion.Type {
	case suggestions.TypeLocalModified:
		borderColor = ui.Warning
		icon = "[UP]"
	case suggestions.TypeRemoteUpdated:
		borderColor = ui.Secondary
		icon = "[DN]"
	case suggestions.TypeConflicts:
		borderColor = ui.Error
		icon = "[!!]"
	case suggestions.TypeFirstRun:
		borderColor = ui.Primary
		icon = "[HI]"
	case suggestions.TypeAllSynced:
		borderColor = ui.Success
		icon = "[OK]"
	default:
		borderColor = ui.Muted
		icon = "[--]"
	}

	// Build content
	b.WriteString("  ")
	b.WriteString(s.renderIcon(icon, s.Suggestion.Type))
	b.WriteString(" ")
	b.WriteString(s.Suggestion.Message)

	// Add action buttons
	if len(s.Suggestion.Actions) > 0 {
		b.WriteString("   ")
		for i, action := range s.Suggestion.Actions {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(s.renderAction(action))
		}
	}

	// Create styled box
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(s.Width - 2)

	return style.Render(b.String())
}

// renderIcon renders the icon with appropriate styling
func (s *SuggestionBar) renderIcon(icon string, suggestionType suggestions.SuggestionType) string {
	var style lipgloss.Style

	switch suggestionType {
	case suggestions.TypeLocalModified:
		style = ui.ModifiedStyle
	case suggestions.TypeRemoteUpdated:
		style = ui.OutdatedStyle
	case suggestions.TypeConflicts:
		style = ui.ConflictStyle
	case suggestions.TypeFirstRun:
		style = lipgloss.NewStyle().Foreground(ui.Primary)
	case suggestions.TypeAllSynced:
		style = ui.SyncedStyle
	default:
		style = ui.MutedStyle
	}

	return style.Bold(true).Render(icon)
}

// renderAction renders an action button
func (s *SuggestionBar) renderAction(action suggestions.Action) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(ui.Foreground).
		Background(ui.Border).
		Padding(0, 1).
		Bold(true)

	labelStyle := ui.MutedStyle

	return fmt.Sprintf("%s %s", keyStyle.Render(action.Key), labelStyle.Render(action.Label))
}

// CompactView renders a compact version for smaller widths
func (s *SuggestionBar) CompactView() string {
	if !s.IsVisible() {
		return ""
	}

	var b strings.Builder

	icon := s.Suggestion.Icon()
	b.WriteString(s.renderIcon(icon, s.Suggestion.Type))
	b.WriteString(" ")

	// Truncate message if needed
	msg := s.Suggestion.Message
	maxLen := s.Width - 20
	if len(msg) > maxLen && maxLen > 0 {
		msg = msg[:maxLen-3] + "..."
	}
	b.WriteString(msg)

	// Show first action only
	if len(s.Suggestion.Actions) > 0 {
		b.WriteString(" ")
		b.WriteString(ui.HelpKeyStyle.Render("[" + s.Suggestion.Actions[0].Key + "]"))
	}

	return ui.MutedStyle.Render(b.String())
}

// Height returns the height of the suggestion bar
func (s *SuggestionBar) Height() int {
	if !s.IsVisible() {
		return 0
	}
	return 3 // Border top + content + border bottom
}

package components

import (
	"fmt"
	"strings"

	"dotsync/internal/modes"
	"dotsync/internal/models"
	"dotsync/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// AppList is a list component for apps
type AppList struct {
	Apps        []*models.App
	Cursor      int
	Width       int
	Height      int
	Focused     bool
	Title       string
	ModesConfig *modes.ModesConfig
}

// NewAppList creates a new app list
func NewAppList(apps []*models.App) *AppList {
	modesCfg, _ := modes.Load()
	return &AppList{
		Apps:        apps,
		Cursor:      0,
		Width:       30,
		Height:      15,
		Focused:     true,
		Title:       "Applications",
		ModesConfig: modesCfg,
	}
}

// SetApps updates the apps list
func (l *AppList) SetApps(apps []*models.App) {
	l.Apps = apps
	if l.Cursor >= len(apps) {
		l.Cursor = max(0, len(apps)-1)
	}
}

// SetModesConfig sets the modes configuration
func (l *AppList) SetModesConfig(cfg *modes.ModesConfig) {
	l.ModesConfig = cfg
}

// ReloadModesConfig reloads modes config from disk
func (l *AppList) ReloadModesConfig() {
	cfg, err := modes.Load()
	if err == nil {
		l.ModesConfig = cfg
	}
}

// MoveUp moves cursor up
func (l *AppList) MoveUp() {
	if l.Cursor > 0 {
		l.Cursor--
	}
}

// MoveDown moves cursor down
func (l *AppList) MoveDown() {
	if l.Cursor < len(l.Apps)-1 {
		l.Cursor++
	}
}

// PageUp moves cursor up by a page
func (l *AppList) PageUp() {
	pageSize := l.Height - 3
	if pageSize < 1 {
		pageSize = 10
	}
	l.Cursor -= pageSize
	if l.Cursor < 0 {
		l.Cursor = 0
	}
}

// PageDown moves cursor down by a page
func (l *AppList) PageDown() {
	pageSize := l.Height - 3
	if pageSize < 1 {
		pageSize = 10
	}
	l.Cursor += pageSize
	if l.Cursor >= len(l.Apps) {
		l.Cursor = max(0, len(l.Apps)-1)
	}
}

// GoToFirst moves cursor to the first item
func (l *AppList) GoToFirst() {
	l.Cursor = 0
}

// GoToLast moves cursor to the last item
func (l *AppList) GoToLast() {
	if len(l.Apps) > 0 {
		l.Cursor = len(l.Apps) - 1
	}
}

// Toggle toggles selection of current item
func (l *AppList) Toggle() {
	if len(l.Apps) > 0 && l.Cursor < len(l.Apps) {
		l.Apps[l.Cursor].ToggleSelected()
	}
}

// SelectAll selects all apps
func (l *AppList) SelectAll() {
	for _, app := range l.Apps {
		app.Selected = true
	}
}

// DeselectAll deselects all apps
func (l *AppList) DeselectAll() {
	for _, app := range l.Apps {
		app.Selected = false
	}
}

// Current returns the currently selected app
func (l *AppList) Current() *models.App {
	if len(l.Apps) > 0 && l.Cursor < len(l.Apps) {
		return l.Apps[l.Cursor]
	}
	return nil
}

// SelectedApps returns all selected apps
func (l *AppList) SelectedApps() []*models.App {
	var selected []*models.App
	for _, app := range l.Apps {
		if app.Selected {
			selected = append(selected, app)
		}
	}
	return selected
}

// VisibleApps returns all apps currently visible in the list
func (l *AppList) VisibleApps() []*models.App {
	return l.Apps
}

// View renders the app list
func (l *AppList) View() string {
	var b strings.Builder

	// Title with counts
	selectedCount := 0
	for _, app := range l.Apps {
		if app.Selected {
			selectedCount++
		}
	}

	title := l.Title
	if selectedCount > 0 {
		title = fmt.Sprintf("%s (%d/%d)", l.Title, selectedCount, len(l.Apps))
	} else if len(l.Apps) > 0 {
		title = fmt.Sprintf("%s (%d)", l.Title, len(l.Apps))
	}
	b.WriteString(ui.PanelTitleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(ui.DividerStyle.Render(strings.Repeat("─", l.Width-2)))
	b.WriteString("\n")

	if len(l.Apps) == 0 {
		b.WriteString(ui.ItemStyle.Render("No apps found"))
		return l.wrapInPanel(b.String())
	}

	// Calculate visible range
	visibleHeight := l.Height - 3 // Minus title and divider
	startIdx := 0
	if l.Cursor >= visibleHeight {
		startIdx = l.Cursor - visibleHeight + 1
	}
	endIdx := min(startIdx+visibleHeight, len(l.Apps))

	// Show scroll indicator at top
	if startIdx > 0 {
		b.WriteString(ui.MutedStyle.Render("  ↑ more"))
		b.WriteString("\n")
	}

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		app := l.Apps[i]
		line := l.renderItem(app, i == l.Cursor)
		b.WriteString(line)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// Show scroll indicator at bottom with position info
	if endIdx < len(l.Apps) {
		b.WriteString("\n")
		b.WriteString(ui.MutedStyle.Render("  ↓ more"))
	}

	// Add position indicator when scrolling
	if len(l.Apps) > visibleHeight {
		position := fmt.Sprintf(" %d/%d ", l.Cursor+1, len(l.Apps))
		b.WriteString("\n")
		b.WriteString(ui.MutedStyle.Render(strings.Repeat(" ", (l.Width-len(position)-4)/2) + position))
	}

	return l.wrapInPanel(b.String())
}

// renderItem renders a single app item
func (l *AppList) renderItem(app *models.App, isCursor bool) string {
	checkbox := ui.RenderCheckbox(app.Selected)
	icon := app.Icon
	if icon == "" {
		icon = "pkg"
	}

	name := app.Name
	maxNameLen := l.Width - 22 // Extra space for mode indicator
	if maxNameLen < 10 {
		maxNameLen = 10
	}
	if len(name) > maxNameLen {
		name = name[:maxNameLen-3] + "..."
	}

	filesCount := fmt.Sprintf("(%d)", len(app.Files))

	// Mode indicator [B] or [B+S]
	modeIndicator := "[B]" // Default to backup only
	modeStyle := ui.MutedStyle
	if l.ModesConfig != nil {
		label := l.ModesConfig.AppSyncLabel(app.ID)
		modeIndicator = "[" + label + "]"
		if l.ModesConfig.IsAppSynced(app.ID) {
			modeStyle = ui.SyncedStyle
		}
	}

	// Count modified/conflict files for status indicator
	var statusIndicator string
	modifiedCount := 0
	conflictCount := 0
	for _, file := range app.Files {
		switch file.ConflictType {
		case models.ConflictLocalModified, models.ConflictLocalNew:
			modifiedCount++
		case models.ConflictBothModified:
			conflictCount++
		}
	}

	if conflictCount > 0 {
		statusIndicator = ui.ConflictStyle.Render("!!")
	} else if modifiedCount > 0 {
		statusIndicator = ui.ModifiedStyle.Render("*")
	}

	content := fmt.Sprintf("%s %s %s %s %s %s", checkbox, icon, name, ui.MutedStyle.Render(filesCount), modeStyle.Render(modeIndicator), statusIndicator)

	if isCursor && l.Focused {
		return ui.SelectedItemStyle.Width(l.Width - 4).Render(content)
	}
	return ui.ItemStyle.Render(content)
}

// wrapInPanel wraps content in a panel border
func (l *AppList) wrapInPanel(content string) string {
	style := ui.PanelStyle
	if l.Focused {
		style = ui.ActivePanelStyle
	}
	return style.Width(l.Width).Height(l.Height).Render(content)
}

// MutedStyle for scroll indicators
var MutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

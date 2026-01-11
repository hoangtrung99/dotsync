package components

import (
	"fmt"
	"strings"

	"dotsync/internal/models"
	"dotsync/internal/ui"
)

// FileList is a list component for files
type FileList struct {
	Files   []models.File
	Cursor  int
	Width   int
	Height  int
	Focused bool
	Title   string
	AppName string
}

// NewFileList creates a new file list
func NewFileList() *FileList {
	return &FileList{
		Files:   []models.File{},
		Cursor:  0,
		Width:   40,
		Height:  15,
		Focused: false,
		Title:   "📄 Files",
	}
}

// SetFiles updates the files list
func (l *FileList) SetFiles(files []models.File, appName string) {
	l.Files = files
	l.AppName = appName
	l.Cursor = 0
}

// Clear clears the file list
func (l *FileList) Clear() {
	l.Files = []models.File{}
	l.AppName = ""
	l.Cursor = 0
}

// MoveUp moves cursor up
func (l *FileList) MoveUp() {
	if l.Cursor > 0 {
		l.Cursor--
	}
}

// MoveDown moves cursor down
func (l *FileList) MoveDown() {
	if l.Cursor < len(l.Files)-1 {
		l.Cursor++
	}
}

// PageUp moves cursor up by a page
func (l *FileList) PageUp() {
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
func (l *FileList) PageDown() {
	pageSize := l.Height - 3
	if pageSize < 1 {
		pageSize = 10
	}
	l.Cursor += pageSize
	if l.Cursor >= len(l.Files) {
		l.Cursor = max(0, len(l.Files)-1)
	}
}

// GoToFirst moves cursor to the first item
func (l *FileList) GoToFirst() {
	l.Cursor = 0
}

// GoToLast moves cursor to the last item
func (l *FileList) GoToLast() {
	if len(l.Files) > 0 {
		l.Cursor = len(l.Files) - 1
	}
}

// Toggle toggles selection of current file
func (l *FileList) Toggle() {
	if len(l.Files) > 0 && l.Cursor < len(l.Files) {
		l.Files[l.Cursor].ToggleSelected()
	}
}

// SelectAll selects all files
func (l *FileList) SelectAll() {
	for i := range l.Files {
		l.Files[i].Selected = true
	}
}

// DeselectAll deselects all files
func (l *FileList) DeselectAll() {
	for i := range l.Files {
		l.Files[i].Selected = false
	}
}

// Current returns the currently selected file
func (l *FileList) Current() *models.File {
	if len(l.Files) > 0 && l.Cursor < len(l.Files) {
		return &l.Files[l.Cursor]
	}
	return nil
}

// SelectedFiles returns all selected files
func (l *FileList) SelectedFiles() []models.File {
	var selected []models.File
	for _, f := range l.Files {
		if f.Selected {
			selected = append(selected, f)
		}
	}
	return selected
}

// View renders the file list
func (l *FileList) View() string {
	var b strings.Builder

	// Title with app name and counts
	selectedCount := 0
	for _, f := range l.Files {
		if f.Selected {
			selectedCount++
		}
	}

	title := l.Title
	if l.AppName != "" {
		if selectedCount > 0 {
			title = fmt.Sprintf("📄 %s (%d/%d)", l.AppName, selectedCount, len(l.Files))
		} else if len(l.Files) > 0 {
			title = fmt.Sprintf("📄 %s (%d)", l.AppName, len(l.Files))
		} else {
			title = fmt.Sprintf("📄 %s", l.AppName)
		}
	}
	b.WriteString(ui.PanelTitleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(ui.DividerStyle.Render(strings.Repeat("─", l.Width-2)))
	b.WriteString("\n")

	if len(l.Files) == 0 {
		b.WriteString(ui.ItemStyle.Render("Select an app to see files"))
		return l.wrapInPanel(b.String())
	}

	// Calculate visible range
	visibleHeight := l.Height - 3
	startIdx := 0
	if l.Cursor >= visibleHeight {
		startIdx = l.Cursor - visibleHeight + 1
	}
	endIdx := min(startIdx+visibleHeight, len(l.Files))

	// Show scroll indicator at top
	if startIdx > 0 {
		b.WriteString(MutedStyle.Render("  ↑ more"))
		b.WriteString("\n")
	}

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		file := l.Files[i]
		line := l.renderItem(&file, i == l.Cursor)
		b.WriteString(line)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// Show scroll indicator at bottom with position info
	if endIdx < len(l.Files) {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("  ↓ more"))
	}

	// Add position indicator when scrolling
	if len(l.Files) > visibleHeight {
		position := fmt.Sprintf(" %d/%d ", l.Cursor+1, len(l.Files))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(strings.Repeat(" ", (l.Width-len(position)-4)/2) + position))
	}

	return l.wrapInPanel(b.String())
}

// renderItem renders a single file item
func (l *FileList) renderItem(file *models.File, isCursor bool) string {
	checkbox := ui.RenderCheckbox(file.Selected)
	icon := file.Icon()

	name := file.RelPath
	if name == "" {
		name = file.Name
	}
	maxNameLen := l.Width - 15
	if len(name) > maxNameLen {
		name = "..." + name[len(name)-maxNameLen+3:]
	}

	// Add encrypted indicator
	suffix := ""
	if file.Encrypted {
		suffix = " " + ui.EncryptedStyle.Render("🔒")
	}

	// Use ConflictType for status display (hash-based, more accurate)
	// Fall back to SyncStatus if ConflictType is not set
	statusIcon := file.ConflictType.ConflictIcon()
	var statusStyle = ui.SyncedStyle
	switch file.ConflictType {
	case models.ConflictLocalModified, models.ConflictLocalNew:
		statusStyle = ui.ModifiedStyle
	case models.ConflictDotfilesModified, models.ConflictDotfilesNew:
		statusStyle = ui.OutdatedStyle
	case models.ConflictBothModified:
		statusStyle = ui.ConflictStyle
	case models.ConflictLocalDeleted, models.ConflictDotfilesDeleted:
		statusStyle = ui.MissingStyle
	case models.ConflictNone:
		statusStyle = ui.SyncedStyle
	default:
		// Fallback to SyncStatus if ConflictType is not set
		statusIcon = file.SyncStatus.StatusIcon()
		switch file.SyncStatus {
		case models.StatusModified:
			statusStyle = ui.ModifiedStyle
		case models.StatusNew:
			statusStyle = ui.NewStyle
		case models.StatusMissing:
			statusStyle = ui.MissingStyle
		}
	}

	content := fmt.Sprintf("%s %s %s%s %s",
		checkbox,
		icon,
		ui.FileNameStyle.Render(name),
		suffix,
		statusStyle.Render(statusIcon),
	)

	if isCursor && l.Focused {
		return ui.SelectedItemStyle.Width(l.Width - 4).Render(content)
	}
	return ui.ItemStyle.Render(content)
}

// wrapInPanel wraps content in a panel border
func (l *FileList) wrapInPanel(content string) string {
	style := ui.PanelStyle
	if l.Focused {
		style = ui.ActivePanelStyle
	}
	return style.Width(l.Width).Height(l.Height).Render(content)
}

package components

import (
	"fmt"
	"strings"
	"time"

	"dotsync/internal/ui"
)

// Machine represents a machine with backup data
type Machine struct {
	Name     string
	LastSync time.Time
	Files    []string
}

// RestoreDialog is a dialog for restoring files from another machine
type RestoreDialog struct {
	Machines       []Machine
	Files          []RestoreFile
	MachineCursor  int
	FileCursor     int
	Width          int
	Height         int
	Step           RestoreStep // 0 = select machine, 1 = select files
	SelectedFiles  map[string]bool
	Visible        bool
}

// RestoreFile represents a file available for restore
type RestoreFile struct {
	Path     string
	Selected bool
}

// RestoreStep represents the current step in restore dialog
type RestoreStep int

const (
	StepSelectMachine RestoreStep = iota
	StepSelectFiles
)

// NewRestoreDialog creates a new restore dialog
func NewRestoreDialog() *RestoreDialog {
	return &RestoreDialog{
		Machines:      []Machine{},
		Files:         []RestoreFile{},
		MachineCursor: 0,
		FileCursor:    0,
		Width:         60,
		Height:        20,
		Step:          StepSelectMachine,
		SelectedFiles: make(map[string]bool),
		Visible:       false,
	}
}

// Show shows the dialog with machine list
func (d *RestoreDialog) Show(machines []Machine) {
	d.Machines = machines
	d.MachineCursor = 0
	d.FileCursor = 0
	d.Step = StepSelectMachine
	d.SelectedFiles = make(map[string]bool)
	d.Visible = true
}

// Hide hides the dialog
func (d *RestoreDialog) Hide() {
	d.Visible = false
}

// IsVisible returns whether the dialog is visible
func (d *RestoreDialog) IsVisible() bool {
	return d.Visible
}

// MoveUp moves cursor up
func (d *RestoreDialog) MoveUp() {
	if d.Step == StepSelectMachine {
		if d.MachineCursor > 0 {
			d.MachineCursor--
		}
	} else {
		if d.FileCursor > 0 {
			d.FileCursor--
		}
	}
}

// MoveDown moves cursor down
func (d *RestoreDialog) MoveDown() {
	if d.Step == StepSelectMachine {
		if d.MachineCursor < len(d.Machines)-1 {
			d.MachineCursor++
		}
	} else {
		if d.FileCursor < len(d.Files)-1 {
			d.FileCursor++
		}
	}
}

// Toggle toggles selection of current item
func (d *RestoreDialog) Toggle() {
	if d.Step == StepSelectFiles && len(d.Files) > 0 {
		file := &d.Files[d.FileCursor]
		file.Selected = !file.Selected
		d.SelectedFiles[file.Path] = file.Selected
	}
}

// SelectAll selects all files
func (d *RestoreDialog) SelectAll() {
	for i := range d.Files {
		d.Files[i].Selected = true
		d.SelectedFiles[d.Files[i].Path] = true
	}
}

// DeselectAll deselects all files
func (d *RestoreDialog) DeselectAll() {
	for i := range d.Files {
		d.Files[i].Selected = false
		d.SelectedFiles[d.Files[i].Path] = false
	}
}

// Confirm confirms current selection and moves to next step or returns result
func (d *RestoreDialog) Confirm() (machineName string, files []string, done bool) {
	if d.Step == StepSelectMachine {
		if len(d.Machines) > 0 {
			// Move to file selection step
			machine := d.Machines[d.MachineCursor]
			d.Files = make([]RestoreFile, len(machine.Files))
			for i, f := range machine.Files {
				d.Files[i] = RestoreFile{Path: f, Selected: true}
				d.SelectedFiles[f] = true
			}
			d.FileCursor = 0
			d.Step = StepSelectFiles
		}
		return "", nil, false
	}

	// Step is StepSelectFiles - return selected files
	machine := d.Machines[d.MachineCursor]
	var selectedFiles []string
	for _, f := range d.Files {
		if f.Selected {
			selectedFiles = append(selectedFiles, f.Path)
		}
	}
	return machine.Name, selectedFiles, true
}

// Back goes back to previous step
func (d *RestoreDialog) Back() bool {
	if d.Step == StepSelectFiles {
		d.Step = StepSelectMachine
		return false // Don't close dialog
	}
	return true // Close dialog
}

// SelectedMachine returns the currently selected machine
func (d *RestoreDialog) SelectedMachine() *Machine {
	if len(d.Machines) > 0 && d.MachineCursor < len(d.Machines) {
		return &d.Machines[d.MachineCursor]
	}
	return nil
}

// GetSelectedFiles returns list of selected file paths
func (d *RestoreDialog) GetSelectedFiles() []string {
	var files []string
	for _, f := range d.Files {
		if f.Selected {
			files = append(files, f.Path)
		}
	}
	return files
}

// View renders the dialog
func (d *RestoreDialog) View() string {
	if !d.Visible {
		return ""
	}

	var b strings.Builder

	// Title
	title := "Restore from another machine"
	b.WriteString(ui.PanelTitleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(ui.DividerStyle.Render(strings.Repeat("-", d.Width-4)))
	b.WriteString("\n\n")

	if d.Step == StepSelectMachine {
		b.WriteString(d.renderMachineSelection())
	} else {
		b.WriteString(d.renderFileSelection())
	}

	// Help bar
	b.WriteString("\n")
	b.WriteString(ui.DividerStyle.Render(strings.Repeat("-", d.Width-4)))
	b.WriteString("\n")
	b.WriteString(d.renderHelp())

	return ui.DialogStyle.Width(d.Width).Render(b.String())
}

// renderMachineSelection renders the machine selection step
func (d *RestoreDialog) renderMachineSelection() string {
	var b strings.Builder

	b.WriteString(ui.MutedStyle.Render("Select source machine:"))
	b.WriteString("\n\n")

	if len(d.Machines) == 0 {
		b.WriteString(ui.MutedStyle.Render("  No other machines found"))
		return b.String()
	}

	for i, machine := range d.Machines {
		prefix := "  "
		if i == d.MachineCursor {
			prefix = "> "
		}

		// Format last sync time
		lastSync := formatTimeAgo(machine.LastSync)
		line := fmt.Sprintf("%s[%d] %s  (%s)", prefix, i+1, machine.Name, lastSync)

		if i == d.MachineCursor {
			b.WriteString(ui.SelectedItemStyle.Width(d.Width - 6).Render(line))
		} else {
			b.WriteString(ui.ItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderFileSelection renders the file selection step
func (d *RestoreDialog) renderFileSelection() string {
	var b strings.Builder

	machine := d.Machines[d.MachineCursor]
	b.WriteString(ui.MutedStyle.Render(fmt.Sprintf("Files from %s:", machine.Name)))
	b.WriteString("\n\n")

	if len(d.Files) == 0 {
		b.WriteString(ui.MutedStyle.Render("  No files available"))
		return b.String()
	}

	// Calculate visible range
	visibleHeight := d.Height - 10
	startIdx := 0
	if d.FileCursor >= visibleHeight {
		startIdx = d.FileCursor - visibleHeight + 1
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(d.Files) {
		endIdx = len(d.Files)
	}

	for i := startIdx; i < endIdx; i++ {
		file := d.Files[i]
		checkbox := ui.RenderCheckbox(file.Selected)
		line := fmt.Sprintf("%s %s", checkbox, file.Path)

		if i == d.FileCursor {
			b.WriteString(ui.SelectedItemStyle.Width(d.Width - 6).Render(line))
		} else {
			b.WriteString(ui.ItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// Selected count
	selectedCount := 0
	for _, f := range d.Files {
		if f.Selected {
			selectedCount++
		}
	}
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render(fmt.Sprintf("Selected: %d/%d files", selectedCount, len(d.Files))))

	return b.String()
}

// renderHelp renders the help bar
func (d *RestoreDialog) renderHelp() string {
	var items []string

	items = append(items, ui.RenderHelpItem("Up/Down", "navigate"))

	if d.Step == StepSelectFiles {
		items = append(items, ui.RenderHelpItem("Space", "select"))
		items = append(items, ui.RenderHelpItem("a", "all"))
	}

	items = append(items, ui.RenderHelpItem("Enter", "confirm"))

	if d.Step == StepSelectFiles {
		items = append(items, ui.RenderHelpItem("Backspace", "back"))
	}

	items = append(items, ui.RenderHelpItem("Esc", "cancel"))

	return strings.Join(items, "  ")
}

// formatTimeAgo formats a time as relative time
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

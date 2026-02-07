package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for the app
type KeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Home        key.Binding
	End         key.Binding
	Tab         key.Binding
	ShiftTab    key.Binding
	Space       key.Binding
	Enter       key.Binding
	SelectAll   key.Binding
	DeselectAll key.Binding
	SelectMod   key.Binding // Select modified apps/files
	SelectOut   key.Binding // Select outdated apps/files (need pull)
	Push        key.Binding // Push local configs to dotfiles
	Pull        key.Binding // Pull configs from dotfiles to local
	Scan        key.Binding
	Brewfile    key.Binding
	Help        key.Binding
	Quit        key.Binding
	Escape      key.Binding
	Diff        key.Binding // View diff for selected file
	Git         key.Binding // Open git panel
	Merge       key.Binding // Open merge tool for conflicts
	NextHunk    key.Binding // Next diff hunk
	PrevHunk    key.Binding // Previous diff hunk
	KeepLocal   key.Binding // Keep local version
	UseDotfiles key.Binding // Use dotfiles version
	Refresh     key.Binding // Refresh current view
	Undo        key.Binding // Undo last selection change
	Preview     key.Binding // Preview file content

	// Quick Sync & Mode keys
	QuickSync     key.Binding // Quick sync (fetch, detect, auto-resolve or open IDE)
	ToggleMode    key.Binding // Toggle mode (Sync <-> Backup)
	SetAllSync    key.Binding // Set all items to Sync mode
	SetAllBackup  key.Binding // Set all items to Backup mode
	Restore       key.Binding // Open restore dialog
	OpenEditor    key.Binding // Open current file in editor
	CheckConflict key.Binding // Check for conflicts
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("PgUp", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("PgDn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("Home", "first"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("End/G", "last"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch panel"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "switch panel"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
		DeselectAll: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "deselect all"),
		),
		Push: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "push to dotfiles"),
		),
		Pull: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "pull from dotfiles"),
		),
		Scan: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "rescan"),
		),
		Brewfile: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "brewfile"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Diff: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "view diff"),
		),
		Git: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "git"),
		),
		Merge: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "merge"),
		),
		NextHunk: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next hunk"),
		),
		PrevHunk: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev hunk"),
		),
		KeepLocal: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "keep local"),
		),
		UseDotfiles: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "use dotfiles"),
		),
		SelectMod: key.NewBinding(
			key.WithKeys("M"),
			key.WithHelp("M", "select modified"),
		),
		SelectOut: key.NewBinding(
			key.WithKeys("O"),
			key.WithHelp("O", "select outdated"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Undo: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "undo"),
		),
		Preview: key.NewBinding(
			key.WithKeys("v", "enter"),
			key.WithHelp("v/enter", "preview"),
		),

		// Quick Sync & Mode keys
		QuickSync: key.NewBinding(
			key.WithKeys("Q"),
			key.WithHelp("Q", "quick sync"),
		),
		ToggleMode: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "toggle mode"),
		),
		SetAllSync: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "set all sync"),
		),
		SetAllBackup: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "set all backup"),
		),
		Restore: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "restore from..."),
		),
		OpenEditor: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "open in editor"),
		),
		CheckConflict: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "check conflicts"),
		),
	}
}

// ShortHelp returns keybindings to show in short help
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Space, k.Tab, k.QuickSync, k.Push, k.Pull, k.Help, k.Quit}
}

// FullHelp returns all keybindings for full help
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		// Panel & Selection
		{k.Tab, k.Space, k.Enter, k.SelectAll, k.DeselectAll},
		// Quick Selection
		{k.SelectMod, k.SelectOut, k.Refresh, k.Undo},
		// Quick Sync & Mode
		{k.QuickSync, k.ToggleMode, k.SetAllSync, k.SetAllBackup},
		// Sync Operations
		{k.Push, k.Pull, k.Scan, k.Brewfile, k.Restore},
		// Diff & Merge
		{k.Diff, k.Merge, k.OpenEditor, k.CheckConflict},
		// Git & General
		{k.Git, k.Help, k.Escape, k.Quit},
	}
}

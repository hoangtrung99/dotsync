package ui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	Primary    = lipgloss.Color("#7C3AED") // Purple
	Secondary  = lipgloss.Color("#06B6D4") // Cyan
	Success    = lipgloss.Color("#10B981") // Green
	Warning    = lipgloss.Color("#F59E0B") // Amber
	Error      = lipgloss.Color("#EF4444") // Red
	Muted      = lipgloss.Color("#6B7280") // Gray
	Background = lipgloss.Color("#1F2937") // Dark gray
	Foreground = lipgloss.Color("#F9FAFB") // Light
	Border     = lipgloss.Color("#374151") // Border gray
	Highlight  = lipgloss.Color("#8B5CF6") // Light purple
	Selected   = lipgloss.Color("#4F46E5") // Indigo
)

// Styles
var (
	// App container
	AppStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			Padding(0, 1).
			MarginBottom(1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Foreground)

	VersionStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	// Panels
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(0, 1)

	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Secondary).
			Padding(0, 1)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Primary).
				Padding(0, 1)

	// List items
	ItemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	SelectedItemStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(Selected).
				Foreground(Foreground)

	CursorStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// Checkbox
	CheckboxChecked   = lipgloss.NewStyle().Foreground(Success).Render("[✓]")
	CheckboxUnchecked = lipgloss.NewStyle().Foreground(Muted).Render("[ ]")

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Padding(0, 1).
			MarginTop(1)

	StatusTextStyle = lipgloss.NewStyle().
			Foreground(Foreground)

	// Help bar
	HelpBarStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Secondary).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Category header
	CategoryStyle = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true).
			Padding(0, 1)

	// File specific
	FileNameStyle = lipgloss.NewStyle().
			Foreground(Foreground)

	FilePathStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	FileSizeStyle = lipgloss.NewStyle().
			Foreground(Muted)

	EncryptedStyle = lipgloss.NewStyle().
			Foreground(Warning)

	// Sync status
	SyncedStyle = lipgloss.NewStyle().
			Foreground(Success)

	ModifiedStyle = lipgloss.NewStyle().
			Foreground(Warning)

	NewStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	MissingStyle = lipgloss.NewStyle().
			Foreground(Error)

	OutdatedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")) // Light blue for outdated

	ConflictStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F472B6")). // Pink for conflicts
			Bold(true)

	// Muted text
	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Progress
	ProgressStyle = lipgloss.NewStyle().
			Foreground(Primary)

	// Divider
	DividerStyle = lipgloss.NewStyle().
			Foreground(Border)

	// Notification/Toast styles
	SuccessNotifyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Background(lipgloss.Color("#064E3B")).
				Padding(0, 1).
				Bold(true)

	ErrorNotifyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FCA5A5")).
				Background(lipgloss.Color("#7F1D1D")).
				Padding(0, 1).
				Bold(true)

	WarningNotifyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FCD34D")).
				Background(lipgloss.Color("#78350F")).
				Padding(0, 1).
				Bold(true)

	InfoNotifyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#93C5FD")).
			Background(lipgloss.Color("#1E3A5F")).
			Padding(0, 1).
			Bold(true)

	// Dialog box style
	DialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2).
			Width(60)

	// Button styles
	ButtonStyle = lipgloss.NewStyle().
			Foreground(Foreground).
			Background(Border).
			Padding(0, 2)

	ButtonActiveStyle = lipgloss.NewStyle().
				Foreground(Foreground).
				Background(Primary).
				Padding(0, 2).
				Bold(true)
)

// RenderCheckbox returns a styled checkbox
func RenderCheckbox(checked bool) string {
	if checked {
		return CheckboxChecked
	}
	return CheckboxUnchecked
}

// RenderHelpItem renders a help key-description pair
func RenderHelpItem(key, desc string) string {
	return HelpKeyStyle.Render(key) + " " + HelpDescStyle.Render(desc)
}

// JoinHorizontal joins strings horizontally with spacing
func JoinHorizontal(left, right string, width int) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

// RenderNotification renders a styled notification message
func RenderNotification(msgType string, message string) string {
	var icon string
	var style lipgloss.Style

	switch msgType {
	case "success":
		icon = "✓"
		style = SuccessNotifyStyle
	case "error":
		icon = "✗"
		style = ErrorNotifyStyle
	case "warning":
		icon = "⚠"
		style = WarningNotifyStyle
	case "info":
		icon = "ℹ"
		style = InfoNotifyStyle
	default:
		icon = "•"
		style = MutedStyle
	}

	return style.Render(icon + " " + message)
}

// RenderButton renders a styled button
func RenderButton(label string, active bool) string {
	if active {
		return ButtonActiveStyle.Render(label)
	}
	return ButtonStyle.Render(label)
}

package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestColors(t *testing.T) {
	colors := []lipgloss.Color{
		Primary, Secondary, Success, Warning, Error,
		Muted, Background, Foreground, Border, Highlight, Selected,
	}

	for _, c := range colors {
		if c == "" {
			t.Error("Color should not be empty")
		}
	}
}

func TestAppStyle(t *testing.T) {
	rendered := AppStyle.Render("test")
	if rendered == "" {
		t.Error("AppStyle should render content")
	}
}

func TestHeaderStyle(t *testing.T) {
	rendered := HeaderStyle.Render("Header")
	if rendered == "" {
		t.Error("HeaderStyle should render content")
	}
}

func TestTitleStyle(t *testing.T) {
	rendered := TitleStyle.Render("Title")
	if rendered == "" {
		t.Error("TitleStyle should render content")
	}
}

func TestVersionStyle(t *testing.T) {
	rendered := VersionStyle.Render("v1.0.0")
	if rendered == "" {
		t.Error("VersionStyle should render content")
	}
}

func TestPanelStyle(t *testing.T) {
	rendered := PanelStyle.Render("Panel content")
	if rendered == "" {
		t.Error("PanelStyle should render content")
	}
}

func TestPanelTitleStyle(t *testing.T) {
	rendered := PanelTitleStyle.Render("Panel Title")
	if rendered == "" {
		t.Error("PanelTitleStyle should render content")
	}
}

func TestActivePanelStyle(t *testing.T) {
	rendered := ActivePanelStyle.Render("Active Panel")
	if rendered == "" {
		t.Error("ActivePanelStyle should render content")
	}
}

func TestItemStyle(t *testing.T) {
	rendered := ItemStyle.Render("Item")
	if rendered == "" {
		t.Error("ItemStyle should render content")
	}
}

func TestSelectedItemStyle(t *testing.T) {
	rendered := SelectedItemStyle.Render("Selected Item")
	if rendered == "" {
		t.Error("SelectedItemStyle should render content")
	}
}

func TestCursorStyle(t *testing.T) {
	rendered := CursorStyle.Render(">")
	if rendered == "" {
		t.Error("CursorStyle should render content")
	}
}

func TestCheckboxStyles(t *testing.T) {
	if CheckboxChecked == "" {
		t.Error("CheckboxChecked should not be empty")
	}
	if CheckboxUnchecked == "" {
		t.Error("CheckboxUnchecked should not be empty")
	}
}

func TestStatusBarStyle(t *testing.T) {
	rendered := StatusBarStyle.Render("Status")
	if rendered == "" {
		t.Error("StatusBarStyle should render content")
	}
}

func TestStatusTextStyle(t *testing.T) {
	rendered := StatusTextStyle.Render("Status text")
	if rendered == "" {
		t.Error("StatusTextStyle should render content")
	}
}

func TestHelpBarStyle(t *testing.T) {
	rendered := HelpBarStyle.Render("Help bar")
	if rendered == "" {
		t.Error("HelpBarStyle should render content")
	}
}

func TestHelpKeyStyle(t *testing.T) {
	rendered := HelpKeyStyle.Render("q")
	if rendered == "" {
		t.Error("HelpKeyStyle should render content")
	}
}

func TestHelpDescStyle(t *testing.T) {
	rendered := HelpDescStyle.Render("quit")
	if rendered == "" {
		t.Error("HelpDescStyle should render content")
	}
}

func TestCategoryStyle(t *testing.T) {
	rendered := CategoryStyle.Render("Category")
	if rendered == "" {
		t.Error("CategoryStyle should render content")
	}
}

func TestFileNameStyle(t *testing.T) {
	rendered := FileNameStyle.Render("config.txt")
	if rendered == "" {
		t.Error("FileNameStyle should render content")
	}
}

func TestFilePathStyle(t *testing.T) {
	rendered := FilePathStyle.Render("/path/to/file")
	if rendered == "" {
		t.Error("FilePathStyle should render content")
	}
}

func TestFileSizeStyle(t *testing.T) {
	rendered := FileSizeStyle.Render("1.2 KB")
	if rendered == "" {
		t.Error("FileSizeStyle should render content")
	}
}

func TestEncryptedStyle(t *testing.T) {
	rendered := EncryptedStyle.Render("🔒")
	if rendered == "" {
		t.Error("EncryptedStyle should render content")
	}
}

func TestSyncedStyle(t *testing.T) {
	rendered := SyncedStyle.Render("✓")
	if rendered == "" {
		t.Error("SyncedStyle should render content")
	}
}

func TestModifiedStyle(t *testing.T) {
	rendered := ModifiedStyle.Render("●")
	if rendered == "" {
		t.Error("ModifiedStyle should render content")
	}
}

func TestNewStyle(t *testing.T) {
	rendered := NewStyle.Render("+")
	if rendered == "" {
		t.Error("NewStyle should render content")
	}
}

func TestMissingStyle(t *testing.T) {
	rendered := MissingStyle.Render("✗")
	if rendered == "" {
		t.Error("MissingStyle should render content")
	}
}

func TestOutdatedStyle(t *testing.T) {
	rendered := OutdatedStyle.Render("○")
	if rendered == "" {
		t.Error("OutdatedStyle should render content")
	}
}

func TestConflictStyle(t *testing.T) {
	rendered := ConflictStyle.Render("⚡")
	if rendered == "" {
		t.Error("ConflictStyle should render content")
	}
}

func TestMutedStyle(t *testing.T) {
	rendered := MutedStyle.Render("muted text")
	if rendered == "" {
		t.Error("MutedStyle should render content")
	}
}

func TestProgressStyle(t *testing.T) {
	rendered := ProgressStyle.Render("50%")
	if rendered == "" {
		t.Error("ProgressStyle should render content")
	}
}

func TestDividerStyle(t *testing.T) {
	rendered := DividerStyle.Render("───")
	if rendered == "" {
		t.Error("DividerStyle should render content")
	}
}

func TestRenderCheckbox(t *testing.T) {
	checked := RenderCheckbox(true)
	if checked == "" {
		t.Error("RenderCheckbox(true) should not be empty")
	}
	if checked != CheckboxChecked {
		t.Error("RenderCheckbox(true) should return CheckboxChecked")
	}

	unchecked := RenderCheckbox(false)
	if unchecked == "" {
		t.Error("RenderCheckbox(false) should not be empty")
	}
	if unchecked != CheckboxUnchecked {
		t.Error("RenderCheckbox(false) should return CheckboxUnchecked")
	}
}

func TestRenderHelpItem(t *testing.T) {
	item := RenderHelpItem("q", "quit")
	if item == "" {
		t.Error("RenderHelpItem should not be empty")
	}
}

func TestJoinHorizontal(t *testing.T) {
	result := JoinHorizontal("left", "right", 80)
	if result == "" {
		t.Error("JoinHorizontal should not be empty")
	}
}

func TestSuccessNotifyStyle(t *testing.T) {
	rendered := SuccessNotifyStyle.Render("Success message")
	if rendered == "" {
		t.Error("SuccessNotifyStyle should render content")
	}
}

func TestErrorNotifyStyle(t *testing.T) {
	rendered := ErrorNotifyStyle.Render("Error message")
	if rendered == "" {
		t.Error("ErrorNotifyStyle should render content")
	}
}

func TestWarningNotifyStyle(t *testing.T) {
	rendered := WarningNotifyStyle.Render("Warning message")
	if rendered == "" {
		t.Error("WarningNotifyStyle should render content")
	}
}

func TestInfoNotifyStyle(t *testing.T) {
	rendered := InfoNotifyStyle.Render("Info message")
	if rendered == "" {
		t.Error("InfoNotifyStyle should render content")
	}
}

func TestDialogStyle(t *testing.T) {
	rendered := DialogStyle.Render("Dialog content")
	if rendered == "" {
		t.Error("DialogStyle should render content")
	}
}

func TestButtonStyle(t *testing.T) {
	rendered := ButtonStyle.Render("Button")
	if rendered == "" {
		t.Error("ButtonStyle should render content")
	}
}

func TestButtonActiveStyle(t *testing.T) {
	rendered := ButtonActiveStyle.Render("Active Button")
	if rendered == "" {
		t.Error("ButtonActiveStyle should render content")
	}
}

func TestRenderNotification(t *testing.T) {
	tests := []struct {
		msgType string
		message string
	}{
		{"success", "Operation completed"},
		{"error", "Something went wrong"},
		{"warning", "Be careful"},
		{"info", "FYI"},
		{"unknown", "Default style"},
	}

	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			result := RenderNotification(tt.msgType, tt.message)
			if result == "" {
				t.Errorf("RenderNotification(%q, %q) should not be empty", tt.msgType, tt.message)
			}
		})
	}
}

func TestRenderButton(t *testing.T) {
	// Test inactive button
	inactive := RenderButton("Cancel", false)
	if inactive == "" {
		t.Error("RenderButton(false) should not be empty")
	}

	// Test active button
	active := RenderButton("OK", true)
	if active == "" {
		t.Error("RenderButton(true) should not be empty")
	}

	// Active and inactive should be different
	if inactive == active {
		t.Error("Active and inactive buttons should render differently")
	}
}

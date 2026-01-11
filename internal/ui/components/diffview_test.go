package components

import (
	"testing"

	"dotsync/internal/sync"
)

func TestNewDiffView(t *testing.T) {
	dv := NewDiffView()

	if dv == nil {
		t.Fatal("NewDiffView should return a DiffView")
	}
	if dv.Width != 80 {
		t.Errorf("Expected width 80, got %d", dv.Width)
	}
	if dv.Height != 20 {
		t.Errorf("Expected height 20, got %d", dv.Height)
	}
	if dv.ScrollOffset != 0 {
		t.Errorf("Expected scrollOffset 0, got %d", dv.ScrollOffset)
	}
}

func TestDiffView_SetDiff(t *testing.T) {
	dv := NewDiffView()
	result := &sync.DiffResult{
		Identical:    false,
		LinesAdded:   5,
		LinesRemoved: 3,
	}

	dv.SetDiff(result, "/local/path", "/dotfiles/path")

	if dv.DiffResult != result {
		t.Error("DiffResult should be set")
	}
	if dv.LocalPath != "/local/path" {
		t.Errorf("Expected /local/path, got %s", dv.LocalPath)
	}
	if dv.DotfilePath != "/dotfiles/path" {
		t.Errorf("Expected /dotfiles/path, got %s", dv.DotfilePath)
	}
	if dv.ScrollOffset != 0 {
		t.Error("ScrollOffset should be reset")
	}
	if dv.CurrentHunk != 0 {
		t.Error("CurrentHunk should be reset")
	}
}

func TestDiffView_ScrollUp(t *testing.T) {
	dv := NewDiffView()
	dv.ScrollOffset = 5

	dv.ScrollUp()
	if dv.ScrollOffset != 4 {
		t.Errorf("Expected 4, got %d", dv.ScrollOffset)
	}

	dv.ScrollOffset = 0
	dv.ScrollUp()
	if dv.ScrollOffset != 0 {
		t.Error("ScrollOffset should not go below 0")
	}
}

func TestDiffView_ScrollDown(t *testing.T) {
	dv := NewDiffView()

	dv.ScrollDown()
	if dv.ScrollOffset != 1 {
		t.Errorf("Expected 1, got %d", dv.ScrollOffset)
	}

	dv.ScrollDown()
	if dv.ScrollOffset != 2 {
		t.Errorf("Expected 2, got %d", dv.ScrollOffset)
	}
}

func TestDiffView_NextHunk(t *testing.T) {
	dv := NewDiffView()
	dv.DiffResult = &sync.DiffResult{
		Hunks: []sync.DiffHunk{{}, {}, {}},
	}

	dv.NextHunk()
	if dv.CurrentHunk != 1 {
		t.Errorf("Expected 1, got %d", dv.CurrentHunk)
	}

	dv.NextHunk()
	if dv.CurrentHunk != 2 {
		t.Errorf("Expected 2, got %d", dv.CurrentHunk)
	}

	// Should not exceed bounds
	dv.NextHunk()
	if dv.CurrentHunk != 2 {
		t.Errorf("Expected 2, got %d", dv.CurrentHunk)
	}
}

func TestDiffView_PrevHunk(t *testing.T) {
	dv := NewDiffView()
	dv.DiffResult = &sync.DiffResult{
		Hunks: []sync.DiffHunk{{}, {}, {}},
	}
	dv.CurrentHunk = 2

	dv.PrevHunk()
	if dv.CurrentHunk != 1 {
		t.Errorf("Expected 1, got %d", dv.CurrentHunk)
	}

	dv.PrevHunk()
	if dv.CurrentHunk != 0 {
		t.Errorf("Expected 0, got %d", dv.CurrentHunk)
	}

	// Should not go below 0
	dv.PrevHunk()
	if dv.CurrentHunk != 0 {
		t.Errorf("Expected 0, got %d", dv.CurrentHunk)
	}
}

func TestDiffView_ToggleHighlight(t *testing.T) {
	dv := NewDiffView()

	// enableHighlight starts as true
	dv.ToggleHighlight()
	// After toggle it should be false, toggle again
	dv.ToggleHighlight()
	// Just verify no panic
}

func TestDiffView_View(t *testing.T) {
	dv := NewDiffView()
	dv.Width = 80
	dv.Height = 20

	// Empty result
	view := dv.View()
	if view == "" {
		t.Error("View should return non-empty string even without result")
	}

	// With result
	dv.DiffResult = &sync.DiffResult{
		Identical:    false,
		LinesAdded:   5,
		LinesRemoved: 3,
		Hunks:        []sync.DiffHunk{{}},
	}
	dv.LocalPath = "/test/local"
	dv.DotfilePath = "/test/dotfiles"

	view = dv.View()
	if view == "" {
		t.Error("View should return non-empty string")
	}
}

func TestDiffView_ViewIdentical(t *testing.T) {
	dv := NewDiffView()
	dv.DiffResult = &sync.DiffResult{
		Identical: true,
	}

	view := dv.View()
	if view == "" {
		t.Error("View should return non-empty string")
	}
}

func TestDiffView_NextHunk_NoResult(t *testing.T) {
	dv := NewDiffView()
	// Should not panic with nil result
	dv.NextHunk()
	dv.PrevHunk()
}

func TestDiffView_HasChanges(t *testing.T) {
	dv := NewDiffView()

	// Nil result
	if dv.HasChanges() {
		t.Error("HasChanges should return false for nil result")
	}

	// Identical files
	dv.DiffResult = &sync.DiffResult{Identical: true}
	if dv.HasChanges() {
		t.Error("HasChanges should return false for identical files")
	}

	// Different files
	dv.DiffResult = &sync.DiffResult{Identical: false}
	if !dv.HasChanges() {
		t.Error("HasChanges should return true for different files")
	}
}

func TestDiffView_HunkCount(t *testing.T) {
	dv := NewDiffView()

	// Nil result
	if dv.HunkCount() != 0 {
		t.Error("HunkCount should return 0 for nil result")
	}

	// Empty hunks
	dv.DiffResult = &sync.DiffResult{Hunks: []sync.DiffHunk{}}
	if dv.HunkCount() != 0 {
		t.Errorf("Expected 0 hunks, got %d", dv.HunkCount())
	}

	// Multiple hunks
	dv.DiffResult = &sync.DiffResult{
		Hunks: []sync.DiffHunk{{}, {}, {}},
	}
	if dv.HunkCount() != 3 {
		t.Errorf("Expected 3 hunks, got %d", dv.HunkCount())
	}
}

func TestDiffView_FormatDiffLine(t *testing.T) {
	dv := NewDiffView()
	dv.DiffResult = &sync.DiffResult{
		OldPath: "test.go",
		NewPath: "test.go",
	}

	// Test insert line
	insertLine := sync.DiffLine{Type: sync.DiffInsert, Content: "added line"}
	result := dv.formatDiffLine(insertLine, 80)
	if result == "" {
		t.Error("formatDiffLine should return non-empty for insert")
	}

	// Test delete line
	deleteLine := sync.DiffLine{Type: sync.DiffDelete, Content: "removed line"}
	result = dv.formatDiffLine(deleteLine, 80)
	if result == "" {
		t.Error("formatDiffLine should return non-empty for delete")
	}

	// Test equal line
	equalLine := sync.DiffLine{Type: sync.DiffEqual, Content: "same line"}
	result = dv.formatDiffLine(equalLine, 80)
	if result == "" {
		t.Error("formatDiffLine should return non-empty for equal")
	}
}

func TestDiffView_FormatDiffLine_LongLine(t *testing.T) {
	dv := NewDiffView()
	dv.DiffResult = &sync.DiffResult{
		OldPath: "test.go",
	}

	// Create a very long line
	longContent := ""
	for i := 0; i < 100; i++ {
		longContent += "a"
	}
	longLine := sync.DiffLine{Type: sync.DiffEqual, Content: longContent}

	// With small max width, should truncate
	result := dv.formatDiffLine(longLine, 50)
	if result == "" {
		t.Error("formatDiffLine should handle long lines")
	}
}

func TestDiffView_FormatDiffLine_WithHighlight(t *testing.T) {
	dv := NewDiffView()
	dv.DiffResult = &sync.DiffResult{
		OldPath: "test.go",
	}
	dv.enableHighlight = true

	equalLine := sync.DiffLine{Type: sync.DiffEqual, Content: "func main() {}"}
	result := dv.formatDiffLine(equalLine, 80)
	if result == "" {
		t.Error("formatDiffLine should handle highlighting")
	}
}

func TestDiffView_ViewWithHunks(t *testing.T) {
	dv := NewDiffView()
	dv.Width = 80
	dv.Height = 30
	dv.DiffResult = &sync.DiffResult{
		Identical: false,
		OldPath:   "old.go",
		NewPath:   "new.go",
		Hunks: []sync.DiffHunk{
			{
				StartOld: 1,
				StartNew: 1,
				DiffLines: []sync.DiffLine{
					{Type: sync.DiffEqual, Content: "line 1", LineNum: 1},
					{Type: sync.DiffDelete, Content: "old line", LineNum: 2},
					{Type: sync.DiffInsert, Content: "new line", LineNum: 2},
				},
			},
		},
	}
	dv.LocalPath = "/local/old.go"
	dv.DotfilePath = "/dotfiles/new.go"

	view := dv.View()
	if view == "" {
		t.Error("View should render hunks")
	}
}

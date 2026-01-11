package components

import (
	"testing"

	"dotsync/internal/sync"
)

func TestNewMergeView(t *testing.T) {
	mv := NewMergeView()

	if mv == nil {
		t.Fatal("NewMergeView should return a MergeView")
	}
	if mv.Width != 80 {
		t.Errorf("Expected width 80, got %d", mv.Width)
	}
	if mv.Height != 20 {
		t.Errorf("Expected height 20, got %d", mv.Height)
	}
	if mv.CurrentHunk != 0 {
		t.Errorf("Expected CurrentHunk 0, got %d", mv.CurrentHunk)
	}
	if mv.ScrollOffset != 0 {
		t.Errorf("Expected ScrollOffset 0, got %d", mv.ScrollOffset)
	}
}

func TestMergeView_SetMerge(t *testing.T) {
	mv := NewMergeView()
	result := &sync.MergeResult{
		TotalHunks: 3,
		Hunks: []sync.MergeHunk{
			{Index: 0},
			{Index: 1},
			{Index: 2},
		},
	}

	mv.CurrentHunk = 2
	mv.ScrollOffset = 5
	mv.SetMerge(result)

	if mv.MergeResult != result {
		t.Error("MergeResult should be set")
	}
	if mv.CurrentHunk != 0 {
		t.Error("CurrentHunk should be reset to 0")
	}
	if mv.ScrollOffset != 0 {
		t.Error("ScrollOffset should be reset to 0")
	}
}

func TestMergeView_NextHunk(t *testing.T) {
	mv := NewMergeView()
	mv.MergeResult = &sync.MergeResult{
		Hunks: []sync.MergeHunk{{}, {}, {}},
	}

	mv.NextHunk()
	if mv.CurrentHunk != 1 {
		t.Errorf("Expected 1, got %d", mv.CurrentHunk)
	}

	mv.NextHunk()
	if mv.CurrentHunk != 2 {
		t.Errorf("Expected 2, got %d", mv.CurrentHunk)
	}

	// Should not exceed bounds
	mv.NextHunk()
	if mv.CurrentHunk != 2 {
		t.Errorf("Expected 2 (no overflow), got %d", mv.CurrentHunk)
	}
}

func TestMergeView_PrevHunk(t *testing.T) {
	mv := NewMergeView()
	mv.MergeResult = &sync.MergeResult{
		Hunks: []sync.MergeHunk{{}, {}, {}},
	}
	mv.CurrentHunk = 2

	mv.PrevHunk()
	if mv.CurrentHunk != 1 {
		t.Errorf("Expected 1, got %d", mv.CurrentHunk)
	}

	mv.PrevHunk()
	if mv.CurrentHunk != 0 {
		t.Errorf("Expected 0, got %d", mv.CurrentHunk)
	}

	// Should not go below 0
	mv.PrevHunk()
	if mv.CurrentHunk != 0 {
		t.Errorf("Expected 0 (no underflow), got %d", mv.CurrentHunk)
	}
}

func TestMergeView_ScrollUp(t *testing.T) {
	mv := NewMergeView()
	mv.ScrollOffset = 5

	mv.ScrollUp()
	if mv.ScrollOffset != 4 {
		t.Errorf("Expected 4, got %d", mv.ScrollOffset)
	}

	mv.ScrollOffset = 0
	mv.ScrollUp()
	if mv.ScrollOffset != 0 {
		t.Error("ScrollOffset should not go below 0")
	}
}

func TestMergeView_ScrollDown(t *testing.T) {
	mv := NewMergeView()

	mv.ScrollDown()
	if mv.ScrollOffset != 1 {
		t.Errorf("Expected 1, got %d", mv.ScrollOffset)
	}

	mv.ScrollDown()
	if mv.ScrollOffset != 2 {
		t.Errorf("Expected 2, got %d", mv.ScrollOffset)
	}
}

func TestMergeView_ResolveCurrentKeepLocal(t *testing.T) {
	mv := NewMergeView()
	mv.MergeResult = &sync.MergeResult{
		TotalHunks: 2,
		Hunks: []sync.MergeHunk{
			{Index: 0, LocalLines: []string{"local1"}, Resolution: sync.ResolutionPending},
			{Index: 1, LocalLines: []string{"local2"}, Resolution: sync.ResolutionPending},
		},
	}

	result := mv.ResolveCurrentKeepLocal()
	if !result {
		t.Error("ResolveCurrentKeepLocal should return true")
	}
	if mv.MergeResult.Hunks[0].Resolution != sync.ResolutionKeepLocal {
		t.Error("Hunk 0 should be resolved as KeepLocal")
	}
}

func TestMergeView_ResolveCurrentUseDotfiles(t *testing.T) {
	mv := NewMergeView()
	mv.MergeResult = &sync.MergeResult{
		TotalHunks: 2,
		Hunks: []sync.MergeHunk{
			{Index: 0, DotfilesLines: []string{"dotfiles1"}, Resolution: sync.ResolutionPending},
			{Index: 1, DotfilesLines: []string{"dotfiles2"}, Resolution: sync.ResolutionPending},
		},
	}

	result := mv.ResolveCurrentUseDotfiles()
	if !result {
		t.Error("ResolveCurrentUseDotfiles should return true")
	}
	if mv.MergeResult.Hunks[0].Resolution != sync.ResolutionUseDotfiles {
		t.Error("Hunk 0 should be resolved as UseDotfiles")
	}
}

func TestMergeView_ResolveCurrentKeepLocal_NoResult(t *testing.T) {
	mv := NewMergeView()
	result := mv.ResolveCurrentKeepLocal()
	if result {
		t.Error("Should return false when no MergeResult")
	}
}

func TestMergeView_ResolveCurrentUseDotfiles_NoResult(t *testing.T) {
	mv := NewMergeView()
	result := mv.ResolveCurrentUseDotfiles()
	if result {
		t.Error("Should return false when no MergeResult")
	}
}

func TestMergeView_IsFullyResolved(t *testing.T) {
	mv := NewMergeView()

	// No result
	if mv.IsFullyResolved() {
		t.Error("Should be false when no MergeResult")
	}

	// With unresolved hunks
	mv.MergeResult = &sync.MergeResult{
		IsFullyResolved: false,
	}
	if mv.IsFullyResolved() {
		t.Error("Should be false when not fully resolved")
	}

	// Fully resolved
	mv.MergeResult.IsFullyResolved = true
	if !mv.IsFullyResolved() {
		t.Error("Should be true when fully resolved")
	}
}

func TestMergeView_HunkCount(t *testing.T) {
	mv := NewMergeView()

	// No result
	if mv.HunkCount() != 0 {
		t.Error("HunkCount should be 0 when no MergeResult")
	}

	// With hunks
	mv.MergeResult = &sync.MergeResult{
		Hunks: []sync.MergeHunk{{}, {}, {}},
	}
	if mv.HunkCount() != 3 {
		t.Errorf("Expected 3, got %d", mv.HunkCount())
	}
}

func TestMergeView_View_NoResult(t *testing.T) {
	mv := NewMergeView()
	view := mv.View()

	if view != "No merge in progress" {
		t.Errorf("Expected 'No merge in progress', got %s", view)
	}
}

func TestMergeView_View_WithResult(t *testing.T) {
	mv := NewMergeView()
	mv.MergeResult = &sync.MergeResult{
		FilePath:   "/test/file.txt",
		LocalPath:  "/local/file.txt",
		TotalHunks: 2,
		Hunks: []sync.MergeHunk{
			{
				Index:         0,
				LocalLines:    []string{"local line"},
				DotfilesLines: []string{"dotfiles line"},
				ContextBefore: []string{"before"},
				ContextAfter:  []string{"after"},
				Resolution:    sync.ResolutionPending,
			},
			{
				Index:      1,
				Resolution: sync.ResolutionKeepLocal,
			},
		},
	}

	view := mv.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestMergeView_View_FullyResolved(t *testing.T) {
	mv := NewMergeView()
	mv.MergeResult = &sync.MergeResult{
		FilePath:        "/test/file.txt",
		TotalHunks:      1,
		ResolvedHunks:   1,
		IsFullyResolved: true,
		Hunks: []sync.MergeHunk{
			{
				Index:      0,
				Resolution: sync.ResolutionKeepLocal,
			},
		},
	}

	view := mv.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestMergeView_AdvanceToNextUnresolved(t *testing.T) {
	mv := NewMergeView()
	mv.MergeResult = &sync.MergeResult{
		TotalHunks: 3,
		Hunks: []sync.MergeHunk{
			{Index: 0, Resolution: sync.ResolutionKeepLocal},
			{Index: 1, Resolution: sync.ResolutionPending},
			{Index: 2, Resolution: sync.ResolutionPending},
		},
	}

	// Start at 0 (resolved), resolve current should advance to 1
	mv.CurrentHunk = 0
	mv.advanceToNextUnresolved()
	if mv.CurrentHunk != 1 {
		t.Errorf("Expected to advance to hunk 1, got %d", mv.CurrentHunk)
	}
}

func TestMergeView_AdvanceToNextUnresolved_WrapAround(t *testing.T) {
	mv := NewMergeView()
	mv.MergeResult = &sync.MergeResult{
		TotalHunks: 3,
		Hunks: []sync.MergeHunk{
			{Index: 0, Resolution: sync.ResolutionPending},
			{Index: 1, Resolution: sync.ResolutionKeepLocal},
			{Index: 2, Resolution: sync.ResolutionKeepLocal},
		},
	}

	// Start at 2, should wrap to 0
	mv.CurrentHunk = 2
	mv.advanceToNextUnresolved()
	if mv.CurrentHunk != 0 {
		t.Errorf("Expected to wrap to hunk 0, got %d", mv.CurrentHunk)
	}
}

func TestMergeView_NextHunk_NoResult(t *testing.T) {
	mv := NewMergeView()
	// Should not panic
	mv.NextHunk()
	mv.PrevHunk()
}

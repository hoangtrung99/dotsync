package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeResolution_String(t *testing.T) {
	tests := []struct {
		res      MergeResolution
		expected string
	}{
		{ResolutionPending, "Pending"},
		{ResolutionKeepLocal, "Keep Local"},
		{ResolutionUseDotfiles, "Use Dotfiles"},
		{ResolutionManual, "Manual"},
		{MergeResolution(99), "Unknown"},
	}

	for _, tc := range tests {
		if tc.res.String() != tc.expected {
			t.Errorf("Expected %s, got %s", tc.expected, tc.res.String())
		}
	}
}

func TestNewMergeResult(t *testing.T) {
	diffResult := &DiffResult{
		OldPath: "/path/to/file",
		Hunks: []DiffHunk{
			{
				StartOld: 1,
				DiffLines: []DiffLine{
					{Type: DiffDelete, Content: "old line"},
					{Type: DiffInsert, Content: "new line"},
				},
			},
		},
	}

	result := NewMergeResult(diffResult, "/local/path", "/dotfiles/path")

	if result.FilePath != "/path/to/file" {
		t.Errorf("Expected FilePath /path/to/file, got %s", result.FilePath)
	}
	if result.LocalPath != "/local/path" {
		t.Errorf("Expected LocalPath /local/path, got %s", result.LocalPath)
	}
	if result.DotfilesPath != "/dotfiles/path" {
		t.Errorf("Expected DotfilesPath /dotfiles/path, got %s", result.DotfilesPath)
	}
	if result.TotalHunks != 1 {
		t.Errorf("Expected TotalHunks 1, got %d", result.TotalHunks)
	}
	if len(result.Hunks) != 1 {
		t.Errorf("Expected 1 hunk, got %d", len(result.Hunks))
	}
}

func TestResolveHunk(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []DiffHunk{
			{
				StartOld: 1,
				DiffLines: []DiffLine{
					{Type: DiffDelete, Content: "local line"},
					{Type: DiffInsert, Content: "dotfiles line"},
				},
			},
		},
	}

	result := NewMergeResult(diffResult, "/local", "/dotfiles")

	// Initially pending
	if result.Hunks[0].Resolution != ResolutionPending {
		t.Error("Initial resolution should be pending")
	}

	// Resolve as keep local
	result.ResolveHunk(0, ResolutionKeepLocal)
	if result.Hunks[0].Resolution != ResolutionKeepLocal {
		t.Error("Resolution should be KeepLocal")
	}
	if len(result.Hunks[0].ResolvedContent) != 1 || result.Hunks[0].ResolvedContent[0] != "local line" {
		t.Error("Resolved content should be local lines")
	}
	if result.ResolvedHunks != 1 {
		t.Errorf("ResolvedHunks should be 1, got %d", result.ResolvedHunks)
	}
	if !result.IsFullyResolved {
		t.Error("Should be fully resolved")
	}

	// Resolve as use dotfiles
	result.ResolveHunk(0, ResolutionUseDotfiles)
	if result.Hunks[0].ResolvedContent[0] != "dotfiles line" {
		t.Error("Resolved content should be dotfiles lines")
	}

	// Test invalid index
	result.ResolveHunk(-1, ResolutionKeepLocal)
	result.ResolveHunk(100, ResolutionKeepLocal)
}

func TestResolveHunkManual(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []DiffHunk{
			{StartOld: 1},
		},
	}

	result := NewMergeResult(diffResult, "/local", "/dotfiles")
	customContent := []string{"custom line 1", "custom line 2"}

	result.ResolveHunkManual(0, customContent)

	if result.Hunks[0].Resolution != ResolutionManual {
		t.Error("Resolution should be Manual")
	}
	if len(result.Hunks[0].ResolvedContent) != 2 {
		t.Error("Should have 2 custom lines")
	}

	// Test invalid index
	result.ResolveHunkManual(-1, customContent)
	result.ResolveHunkManual(100, customContent)
}

func TestKeepAllLocal(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []DiffHunk{
			{
				DiffLines: []DiffLine{
					{Type: DiffDelete, Content: "local1"},
				},
			},
			{
				DiffLines: []DiffLine{
					{Type: DiffDelete, Content: "local2"},
				},
			},
		},
	}

	result := NewMergeResult(diffResult, "/local", "/dotfiles")
	result.KeepAllLocal()

	for i, hunk := range result.Hunks {
		if hunk.Resolution != ResolutionKeepLocal {
			t.Errorf("Hunk %d should be KeepLocal", i)
		}
	}
	if !result.IsFullyResolved {
		t.Error("Should be fully resolved")
	}
}

func TestUseAllDotfiles(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []DiffHunk{
			{
				DiffLines: []DiffLine{
					{Type: DiffInsert, Content: "dotfiles1"},
				},
			},
			{
				DiffLines: []DiffLine{
					{Type: DiffInsert, Content: "dotfiles2"},
				},
			},
		},
	}

	result := NewMergeResult(diffResult, "/local", "/dotfiles")
	result.UseAllDotfiles()

	for i, hunk := range result.Hunks {
		if hunk.Resolution != ResolutionUseDotfiles {
			t.Errorf("Hunk %d should be UseDotfiles", i)
		}
	}
	if !result.IsFullyResolved {
		t.Error("Should be fully resolved")
	}
}

func TestGenerateMergedContent_NotResolved(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []DiffHunk{{StartOld: 1}},
	}

	result := NewMergeResult(diffResult, "/local", "/dotfiles")
	_, err := result.GenerateMergedContent()
	if err == nil {
		t.Error("Should return error when not fully resolved")
	}
}

func TestGenerateMergedContent_FileNotExist(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []DiffHunk{{StartOld: 1}},
	}

	result := NewMergeResult(diffResult, "/nonexistent/path", "/dotfiles")
	result.ResolveHunk(0, ResolutionKeepLocal)
	_, err := result.GenerateMergedContent()
	if err == nil {
		t.Error("Should return error when file doesn't exist")
	}
}

func TestGenerateMergedContent_Success(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	os.WriteFile(localFile, []byte("line1\nline2\nline3"), 0644)

	diffResult := &DiffResult{
		Hunks: []DiffHunk{},
	}

	result := NewMergeResult(diffResult, localFile, "/dotfiles")
	// No hunks means fully resolved
	result.IsFullyResolved = true

	content, err := result.GenerateMergedContent()
	if err != nil {
		t.Errorf("Should not return error: %v", err)
	}
	if content == "" {
		t.Error("Content should not be empty")
	}
}

func TestWriteMergedFile(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	os.WriteFile(localFile, []byte("original content"), 0644)

	diffResult := &DiffResult{
		Hunks: []DiffHunk{},
	}

	result := NewMergeResult(diffResult, localFile, "/dotfiles")
	result.IsFullyResolved = true

	err := result.WriteMergedFile()
	if err != nil {
		t.Errorf("WriteMergedFile failed: %v", err)
	}

	// Verify file was written
	_, err = os.Stat(localFile)
	if err != nil {
		t.Error("File should exist after writing")
	}
}

func TestWriteMergedFile_NotResolved(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []DiffHunk{{StartOld: 1}},
	}

	result := NewMergeResult(diffResult, "/nonexistent", "/dotfiles")
	err := result.WriteMergedFile()
	if err == nil {
		t.Error("Should return error when not resolved")
	}
}

func TestFormatHunkPreview(t *testing.T) {
	hunk := MergeHunk{
		Index:         0,
		LocalLines:    []string{"local line 1", "local line 2"},
		DotfilesLines: []string{"dotfiles line 1"},
	}

	preview := hunk.FormatHunkPreview(0)
	if preview == "" {
		t.Error("Preview should not be empty")
	}

	// Test with max lines limit
	preview = hunk.FormatHunkPreview(1)
	if preview == "" {
		t.Error("Preview should not be empty")
	}
}

func TestMergeHunk_ContextLines(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []DiffHunk{
			{
				StartOld: 1,
				DiffLines: []DiffLine{
					{Type: DiffEqual, Content: "context before"},
					{Type: DiffDelete, Content: "deleted"},
					{Type: DiffInsert, Content: "inserted"},
					{Type: DiffEqual, Content: "context after"},
				},
			},
		},
	}

	result := NewMergeResult(diffResult, "/local", "/dotfiles")

	if len(result.Hunks[0].ContextBefore) != 1 {
		t.Errorf("Expected 1 context before, got %d", len(result.Hunks[0].ContextBefore))
	}
	if len(result.Hunks[0].ContextAfter) != 1 {
		t.Errorf("Expected 1 context after, got %d", len(result.Hunks[0].ContextAfter))
	}
	if len(result.Hunks[0].LocalLines) != 1 {
		t.Errorf("Expected 1 local line, got %d", len(result.Hunks[0].LocalLines))
	}
	if len(result.Hunks[0].DotfilesLines) != 1 {
		t.Errorf("Expected 1 dotfiles line, got %d", len(result.Hunks[0].DotfilesLines))
	}
}

func TestGenerateMergedContent_WithHunks(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	os.WriteFile(localFile, []byte("line1\nline2\nline3\nline4\nline5"), 0644)

	diffResult := &DiffResult{
		Hunks: []DiffHunk{
			{
				StartOld: 2,
				DiffLines: []DiffLine{
					{Type: DiffDelete, Content: "line2"},
					{Type: DiffInsert, Content: "modified line2"},
				},
			},
		},
	}

	result := NewMergeResult(diffResult, localFile, "/dotfiles")
	result.ResolveHunk(0, ResolutionUseDotfiles)

	content, err := result.GenerateMergedContent()
	if err != nil {
		t.Errorf("GenerateMergedContent failed: %v", err)
	}
	if content == "" {
		t.Error("Merged content should not be empty")
	}
}

func TestWriteMergedFile_WithContent(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	os.WriteFile(localFile, []byte("original"), 0644)

	diffResult := &DiffResult{
		Hunks: []DiffHunk{},
	}

	result := NewMergeResult(diffResult, localFile, "/dotfiles")
	result.IsFullyResolved = true
	result.MergedContent = "merged content"

	err := result.WriteMergedFile()
	if err != nil {
		t.Errorf("WriteMergedFile failed: %v", err)
	}

	// Verify content was written
	content, _ := os.ReadFile(localFile)
	if string(content) != "merged content" {
		t.Errorf("Expected 'merged content', got '%s'", string(content))
	}
}

func TestWriteMergedFile_CreateDirectory(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "subdir", "local.txt")

	diffResult := &DiffResult{
		Hunks: []DiffHunk{},
	}

	result := NewMergeResult(diffResult, localFile, "/dotfiles")
	result.IsFullyResolved = true
	result.MergedContent = "content in new dir"

	err := result.WriteMergedFile()
	if err != nil {
		t.Errorf("WriteMergedFile failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(localFile); os.IsNotExist(err) {
		t.Error("File should be created in new directory")
	}
}

func TestFormatHunkPreview_LongContent(t *testing.T) {
	hunk := MergeHunk{
		Index:         0,
		LocalLines:    []string{"local1", "local2", "local3", "local4", "local5"},
		DotfilesLines: []string{"dotfiles1", "dotfiles2", "dotfiles3", "dotfiles4"},
	}

	// Test with max 2 lines
	preview := hunk.FormatHunkPreview(2)
	if preview == "" {
		t.Error("Preview should not be empty")
	}
	// Should contain "... and X more lines"
	if len(hunk.LocalLines) > 2 && !containsString(preview, "more lines") {
		t.Error("Preview should indicate more lines when truncated")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGenerateMergedContent_MultipleHunks(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	os.WriteFile(localFile, []byte("line1\nline2\nline3\nline4\nline5\nline6"), 0644)

	diffResult := &DiffResult{
		Hunks: []DiffHunk{
			{
				StartOld: 2,
				DiffLines: []DiffLine{
					{Type: DiffDelete, Content: "line2"},
					{Type: DiffInsert, Content: "new line2"},
				},
			},
			{
				StartOld: 5,
				DiffLines: []DiffLine{
					{Type: DiffDelete, Content: "line5"},
					{Type: DiffInsert, Content: "new line5"},
				},
			},
		},
	}

	result := NewMergeResult(diffResult, localFile, "/dotfiles")
	result.KeepAllLocal()

	content, err := result.GenerateMergedContent()
	if err != nil {
		t.Errorf("GenerateMergedContent failed: %v", err)
	}
	if content == "" {
		t.Error("Merged content should not be empty")
	}
}

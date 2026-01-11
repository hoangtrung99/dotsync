package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeFileHash(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	content := "Hello, World!"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	hash, err := ComputeFileHash(tmpFile)
	if err != nil {
		t.Fatalf("ComputeFileHash failed: %v", err)
	}

	if hash == "" {
		t.Error("Hash should not be empty")
	}

	// Hash should be consistent
	hash2, _ := ComputeFileHash(tmpFile)
	if hash != hash2 {
		t.Errorf("Hash should be consistent: %s != %s", hash, hash2)
	}
}

func TestQuickHash(t *testing.T) {
	hash1 := QuickHash("test content")
	hash2 := QuickHash("test content")
	hash3 := QuickHash("different content")

	if hash1 != hash2 {
		t.Error("Same content should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("Different content should produce different hash")
	}
}

func TestComputeDiff(t *testing.T) {
	tmpDir := t.TempDir()

	// Create old file
	oldFile := filepath.Join(tmpDir, "old.txt")
	oldContent := "line1\nline2\nline3\n"
	os.WriteFile(oldFile, []byte(oldContent), 0644)

	// Create new file with changes
	newFile := filepath.Join(tmpDir, "new.txt")
	newContent := "line1\nmodified\nline3\nline4\n"
	os.WriteFile(newFile, []byte(newContent), 0644)

	result, err := ComputeDiff(oldFile, newFile)
	if err != nil {
		t.Fatalf("ComputeDiff failed: %v", err)
	}

	if result.Identical {
		t.Error("Files should not be identical")
	}

	if result.LinesAdded == 0 && result.LinesRemoved == 0 {
		t.Error("Should detect changes")
	}
}

func TestComputeDiff_Identical(t *testing.T) {
	tmpDir := t.TempDir()

	content := "same content\n"
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	os.WriteFile(file1, []byte(content), 0644)
	os.WriteFile(file2, []byte(content), 0644)

	result, err := ComputeDiff(file1, file2)
	if err != nil {
		t.Fatalf("ComputeDiff failed: %v", err)
	}

	if !result.Identical {
		t.Error("Files should be identical")
	}
}

func TestComputeDiff_NewFile(t *testing.T) {
	tmpDir := t.TempDir()

	newFile := filepath.Join(tmpDir, "new.txt")
	os.WriteFile(newFile, []byte("new content\n"), 0644)

	nonExistent := filepath.Join(tmpDir, "nonexistent.txt")

	result, err := ComputeDiff(nonExistent, newFile)
	if err != nil {
		t.Fatalf("ComputeDiff failed: %v", err)
	}

	if result.OldExists {
		t.Error("Old file should not exist")
	}

	if !result.NewExists {
		t.Error("New file should exist")
	}

	if result.LinesAdded == 0 {
		t.Error("Should have added lines")
	}
}

func TestReadLines(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	content := "line1\nline2\nline3\n"
	os.WriteFile(tmpFile, []byte(content), 0644)

	lines, err := readLines(tmpFile)
	if err != nil {
		t.Fatalf("readLines failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	if lines[0] != "line1" {
		t.Errorf("Expected 'line1', got '%s'", lines[0])
	}
}

func TestReadLines_NonExistent(t *testing.T) {
	_, err := readLines("/nonexistent/file.txt")
	if err == nil {
		t.Error("readLines should return error for non-existent file")
	}
}

func TestReadLines_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.txt")

	os.WriteFile(tmpFile, []byte(""), 0644)

	lines, err := readLines(tmpFile)
	if err != nil {
		t.Fatalf("readLines failed: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("Expected 0 lines, got %d", len(lines))
	}
}

func TestFormatUnifiedDiff(t *testing.T) {
	result := &DiffResult{
		OldPath: "old.txt",
		NewPath: "new.txt",
		Hunks: []DiffHunk{
			{
				DiffLines: []DiffLine{
					{Type: DiffEqual, Content: "same line"},
					{Type: DiffDelete, Content: "old line"},
					{Type: DiffInsert, Content: "new line"},
				},
			},
		},
	}

	output := FormatUnifiedDiff(result)

	if output == "" {
		t.Error("FormatUnifiedDiff should return non-empty output")
	}

	// Check for expected content
	if !contains(output, "--- old.txt") {
		t.Error("Output should contain old file header")
	}
	if !contains(output, "+++ new.txt") {
		t.Error("Output should contain new file header")
	}
	if !contains(output, " same line") {
		t.Error("Output should contain equal lines with space prefix")
	}
	if !contains(output, "-old line") {
		t.Error("Output should contain deleted lines with - prefix")
	}
	if !contains(output, "+new line") {
		t.Error("Output should contain inserted lines with + prefix")
	}
}

func TestFormatUnifiedDiff_Empty(t *testing.T) {
	result := &DiffResult{
		OldPath: "old.txt",
		NewPath: "new.txt",
		Hunks:   []DiffHunk{},
	}

	output := FormatUnifiedDiff(result)

	// Should still have headers
	if !contains(output, "--- old.txt") {
		t.Error("Output should contain old file header")
	}
}

func TestDiffResult_HasChanges(t *testing.T) {
	// Test with changes
	result := &DiffResult{Identical: false}
	if !result.HasChanges() {
		t.Error("HasChanges should return true when not identical")
	}

	// Test without changes
	result = &DiffResult{Identical: true}
	if result.HasChanges() {
		t.Error("HasChanges should return false when identical")
	}
}

func TestDiffResult_Summary(t *testing.T) {
	// Test with no changes
	result := &DiffResult{Identical: true}
	summary := result.Summary()
	if summary != "No changes" {
		t.Errorf("Expected 'No changes', got '%s'", summary)
	}

	// Test with additions only
	result = &DiffResult{Identical: false, LinesAdded: 5, LinesRemoved: 0}
	summary = result.Summary()
	if summary != "+5" {
		t.Errorf("Expected '+5', got '%s'", summary)
	}

	// Test with deletions only
	result = &DiffResult{Identical: false, LinesAdded: 0, LinesRemoved: 3}
	summary = result.Summary()
	if summary != "-3" {
		t.Errorf("Expected '-3', got '%s'", summary)
	}

	// Test with both additions and deletions
	result = &DiffResult{Identical: false, LinesAdded: 5, LinesRemoved: 3}
	summary = result.Summary()
	if summary != "+5 -3" {
		t.Errorf("Expected '+5 -3', got '%s'", summary)
	}
}

func TestLinesToDiff(t *testing.T) {
	lines := []string{"line1", "line2", "line3"}
	result := linesToDiff(lines, DiffInsert)

	if len(result) != 3 {
		t.Errorf("Expected 3 diff lines, got %d", len(result))
	}

	for i, line := range result {
		if line.Type != DiffInsert {
			t.Errorf("Line %d should be DiffInsert", i)
		}
		if line.LineNum != i+1 {
			t.Errorf("Line %d should have LineNum %d, got %d", i, i+1, line.LineNum)
		}
	}
}

func TestLinesToDiff_Empty(t *testing.T) {
	result := linesToDiff([]string{}, DiffEqual)
	if len(result) != 0 {
		t.Errorf("Expected 0 diff lines, got %d", len(result))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

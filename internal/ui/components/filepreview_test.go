package components

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFilePreview(t *testing.T) {
	fp := NewFilePreview()
	if fp == nil {
		t.Fatal("NewFilePreview should return a FilePreview")
	}
	if fp.Width != 80 {
		t.Errorf("Default width should be 80, got %d", fp.Width)
	}
	if fp.Height != 20 {
		t.Errorf("Default height should be 20, got %d", fp.Height)
	}
}

func TestFilePreview_LoadFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line 1\nline 2\nline 3"
	os.WriteFile(tmpFile, []byte(content), 0644)

	fp := NewFilePreview()
	fp.SetSize(80, 30)
	err := fp.Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if fp.TotalLines != 3 {
		t.Errorf("TotalLines should be 3, got %d", fp.TotalLines)
	}
	if fp.FileName != "test.txt" {
		t.Errorf("FileName should be 'test.txt', got %s", fp.FileName)
	}
}

func TestFilePreview_LoadDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content"), 0644)

	fp := NewFilePreview()
	fp.SetSize(80, 30)
	err := fp.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if fp.TotalLines == 0 {
		t.Error("TotalLines should not be 0 for directory")
	}
}

func TestFilePreview_LoadNonExistent(t *testing.T) {
	fp := NewFilePreview()
	err := fp.Load("/nonexistent/file.txt")
	if err == nil {
		t.Error("Load should fail for non-existent file")
	}
}

func TestFilePreview_Scroll(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	// Create file with many lines
	var content string
	for i := 0; i < 100; i++ {
		content += "line\n"
	}
	os.WriteFile(tmpFile, []byte(content), 0644)

	fp := NewFilePreview()
	fp.SetSize(80, 20)
	fp.Load(tmpFile)

	// Test scroll down - viewport handles internally
	fp.ScrollDown()
	// Just verify no panic occurs

	// Test scroll up
	fp.ScrollUp()
	// Just verify no panic occurs
}

func TestFilePreview_PageNavigation(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	var content string
	for i := 0; i < 100; i++ {
		content += "line\n"
	}
	os.WriteFile(tmpFile, []byte(content), 0644)

	fp := NewFilePreview()
	fp.SetSize(80, 20)
	fp.Load(tmpFile)

	// Test page down - viewport handles internally
	fp.PageDown()
	// Just verify no panic occurs

	// Test page up
	fp.PageUp()
	// Just verify no panic occurs
}

func TestFilePreview_GoToTopBottom(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	var content string
	for i := 0; i < 100; i++ {
		content += "line\n"
	}
	os.WriteFile(tmpFile, []byte(content), 0644)

	fp := NewFilePreview()
	fp.SetSize(80, 20)
	fp.Load(tmpFile)

	// Test go to bottom - viewport handles internally
	fp.GoToBottom()
	// Just verify no panic occurs

	// Test go to top
	fp.GoToTop()
	// Just verify no panic occurs
}

func TestFilePreview_View(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	fp := NewFilePreview()
	fp.SetSize(80, 20)
	fp.Load(tmpFile)

	view := fp.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestFilePreview_SetSize(t *testing.T) {
	fp := NewFilePreview()
	fp.SetSize(100, 50)

	if fp.Width != 100 {
		t.Errorf("Width should be 100, got %d", fp.Width)
	}
	if fp.Height != 50 {
		t.Errorf("Height should be 50, got %d", fp.Height)
	}
}

func TestFilePreview_Update(t *testing.T) {
	fp := NewFilePreview()
	fp.SetSize(80, 20)

	// Test that Update returns without panic
	newFp, cmd := fp.Update(nil)
	if newFp == nil {
		t.Error("Update should return FilePreview")
	}
	_ = cmd // cmd may be nil, that's ok
}

func TestIsBinaryContent(t *testing.T) {
	// Text content
	textData := []byte("Hello, World!\nThis is text.")
	if isBinaryContent(textData) {
		t.Error("Text content should not be detected as binary")
	}

	// Binary content (contains null bytes)
	binaryData := []byte{0x48, 0x65, 0x00, 0x6c, 0x6c, 0x6f}
	if !isBinaryContent(binaryData) {
		t.Error("Binary content with null bytes should be detected")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"no escape codes", "no escape codes"},
	}

	for _, tt := range tests {
		result := stripAnsi(tt.input)
		if result != tt.expected {
			t.Errorf("stripAnsi(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestFilePreview_LoadLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "large.txt")

	// Create a file just under 1MB - should load
	data := make([]byte, 500*1024) // 500KB
	for i := range data {
		data[i] = 'a'
	}
	os.WriteFile(tmpFile, data, 0644)

	fp := NewFilePreview()
	fp.SetSize(80, 20)
	err := fp.Load(tmpFile)
	if err != nil {
		t.Fatalf("Load should succeed for file under 1MB: %v", err)
	}
}

func TestFilePreview_LoadBinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "binary.bin")

	// Create binary file with null bytes
	data := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	os.WriteFile(tmpFile, data, 0644)

	fp := NewFilePreview()
	fp.SetSize(80, 20)
	err := fp.Load(tmpFile)
	if err != nil {
		t.Fatalf("Load should not error for binary file: %v", err)
	}

	// Should show binary message
	view := fp.View()
	if view == "" {
		t.Error("View should not be empty for binary file")
	}
}

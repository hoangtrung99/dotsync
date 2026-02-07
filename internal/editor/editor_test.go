package editor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Editor != "auto" {
		t.Errorf("expected editor to be 'auto', got %s", cfg.Editor)
	}
	if len(cfg.Priority) != 3 {
		t.Errorf("expected 3 editors in priority, got %d", len(cfg.Priority))
	}
	expected := []string{"cursor", "code", "zed"}
	for i, e := range expected {
		if cfg.Priority[i] != e {
			t.Errorf("expected priority[%d] to be %s, got %s", i, e, cfg.Priority[i])
		}
	}
}

func TestIsCommandAvailable(t *testing.T) {
	// Test with a command that should exist on all systems
	if !isCommandAvailable("ls") {
		t.Error("expected 'ls' command to be available")
	}

	// Test with a command that should not exist
	if isCommandAvailable("this-command-does-not-exist-12345") {
		t.Error("expected fake command to not be available")
	}
}

func TestEditorName(t *testing.T) {
	tests := []struct {
		editor Editor
		name   string
	}{
		{NewVSCode(), "VS Code"},
		{NewCursor(), "Cursor"},
		{NewZed(), "Zed"},
	}

	for _, tt := range tests {
		if tt.editor.Name() != tt.name {
			t.Errorf("expected name %s, got %s", tt.name, tt.editor.Name())
		}
	}
}

func TestListInstalled(t *testing.T) {
	// This test just ensures the function runs without error
	editors := ListInstalled()
	// We can't assert specific editors since we don't know what's installed
	t.Logf("Found %d installed editors", len(editors))
	for _, e := range editors {
		t.Logf("  - %s", e.Name())
	}
}

func TestFileWatcher(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(tmpFile, []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	watcher := NewFileWatcher(tmpFile)

	// Test with timeout (file not modified)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := watcher.WaitForChange(ctx)
	if result.Modified {
		t.Error("expected file to not be modified")
	}
	if result.Error != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got %v", result.Error)
	}
}

func TestFileWatcherModification(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(tmpFile, []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	watcher := NewFileWatcher(tmpFile)

	// Modify file in background
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.WriteFile(tmpFile, []byte("modified content!!!"), 0644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := watcher.WaitForChange(ctx)
	if !result.Modified {
		t.Error("expected file to be modified")
	}
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
}

func TestDetectWithUnknownEditor(t *testing.T) {
	cfg := &Config{
		Editor: "unknown-editor",
	}
	_, err := Detect(cfg)
	if err == nil {
		t.Error("expected error for unknown editor")
	}
}

func TestDetectWithNotInstalledEditor(t *testing.T) {
	// Test requesting a specific editor that's not installed
	// We use a modified approach to avoid testing actual installations
	cfg := &Config{
		Editor:   "auto",
		Priority: []string{"nonexistent-editor"},
	}

	// This should still work if any editor is installed, or fail gracefully
	_, _ = Detect(cfg)
	// No assertion needed - we're just checking it doesn't panic
}

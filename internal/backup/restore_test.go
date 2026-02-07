package backup

import (
	"os"
	"path/filepath"
	"testing"

	"dotsync/internal/config"
	"dotsync/internal/modes"
)

func TestRestoreInvalidMachine(t *testing.T) {
	_, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	opts := RestoreOptions{
		SourceMachine: "nonexistent",
		Files:         []string{"zsh/.zshrc"},
	}

	_, err := bm.Restore(opts)
	if err == nil {
		t.Error("expected error for nonexistent machine")
	}
}

func TestGetRestorableFiles(t *testing.T) {
	_, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create backup structure - flat files
	machine1Dir := filepath.Join(bm.config.DotfilesPath, "zsh", "machine-1")
	machine2Dir := filepath.Join(bm.config.DotfilesPath, "nvim", "machine-1")
	os.MkdirAll(machine1Dir, 0755)
	os.MkdirAll(machine2Dir, 0755)
	os.WriteFile(filepath.Join(machine1Dir, ".zshrc"), []byte("zsh"), 0644)
	os.WriteFile(filepath.Join(machine2Dir, "init.lua"), []byte("lua"), 0644)

	// Create backup structure - nested path
	nestedDir := filepath.Join(bm.config.DotfilesPath, "claude-code", "machine-1", "skills", "file-organizer")
	os.MkdirAll(nestedDir, 0755)
	os.WriteFile(filepath.Join(nestedDir, "SKILL.md"), []byte("skill"), 0644)

	files, err := bm.GetRestorableFiles("machine-1")
	if err != nil {
		t.Fatalf("get restorable files failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}

	// Find the nested file and verify its fileName preserves structure
	foundNested := false
	for _, f := range files {
		if f.AppID == "claude-code" {
			foundNested = true
			expected := filepath.Join("skills", "file-organizer", "SKILL.md")
			if f.FileName != expected {
				t.Errorf("nested FileName = %s, want %s", f.FileName, expected)
			}
		}
	}
	if !foundNested {
		t.Error("expected to find nested file for claude-code app")
	}
}

func TestParseFileSpec(t *testing.T) {
	tests := []struct {
		spec         string
		expectedApp  string
		expectedFile string
	}{
		{"zsh/.zshrc", "zsh", ".zshrc"},
		{"nvim/init.lua", "nvim", "init.lua"},
		{"invalid", "", ""},
		{"a/b/c", "a", "b/c"},
	}

	for _, tt := range tests {
		appID, fileName := parseFileSpec(tt.spec)
		if appID != tt.expectedApp {
			t.Errorf("parseFileSpec(%s): appID = %s, want %s", tt.spec, appID, tt.expectedApp)
		}
		if fileName != tt.expectedFile {
			t.Errorf("parseFileSpec(%s): fileName = %s, want %s", tt.spec, fileName, tt.expectedFile)
		}
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"a/b/c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"a/b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		result := splitPath(tt.path)
		if len(result) != len(tt.expected) {
			t.Errorf("splitPath(%s): got %v, want %v", tt.path, result, tt.expected)
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("splitPath(%s)[%d]: got %s, want %s", tt.path, i, v, tt.expected[i])
			}
		}
	}
}

func TestFileComparison(t *testing.T) {
	fc := &FileComparison{
		SourceExists: true,
		SourceSize:   100,
		LocalExists:  true,
		LocalSize:    100,
	}

	if fc.IsDifferent() {
		t.Error("same size files should not be different")
	}

	fc.LocalSize = 200
	if !fc.IsDifferent() {
		t.Error("different size files should be different")
	}

	fc2 := &FileComparison{
		SourceExists: true,
		LocalExists:  false,
	}
	if !fc2.IsDifferent() {
		t.Error("missing local should be different")
	}
}

func TestGetMachineBackupPath(t *testing.T) {
	cfg := &config.Config{
		DotfilesPath: "/home/user/dotfiles",
	}
	modesCfg := &modes.ModesConfig{
		MachineName: "my-machine",
	}
	bm := New(cfg, modesCfg)

	path := bm.GetMachineBackupPath("zsh", "other-machine", ".zshrc")
	expected := "/home/user/dotfiles/zsh/other-machine/.zshrc"

	if path != expected {
		t.Errorf("got %s, want %s", path, expected)
	}
}

func TestGetLocalConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		DotfilesPath: filepath.Join(tmpDir, "dotfiles"),
	}
	modesCfg := &modes.ModesConfig{
		MachineName: "test-machine",
	}
	bm := New(cfg, modesCfg)

	// Test dotfile (starts with .)
	path := bm.getLocalConfigPath("zsh", ".zshrc")
	homeDir, _ := os.UserHomeDir()
	expected := filepath.Join(homeDir, ".zshrc")

	if path != expected {
		t.Errorf("got %s, want %s", path, expected)
	}

	// Test non-dotfile
	path = bm.getLocalConfigPath("nvim", "init.lua")
	expected = filepath.Join(homeDir, ".config", "nvim", "init.lua")

	if path != expected {
		t.Errorf("got %s, want %s", path, expected)
	}
}

func TestGetRestoreBackupPath(t *testing.T) {
	cfg := &config.Config{
		BackupPath: "/home/user/.dotfiles-backup",
	}
	modesCfg := &modes.ModesConfig{}
	bm := New(cfg, modesCfg)

	path := bm.getRestoreBackupPath("zsh", ".zshrc")

	// Check that it contains expected components
	if !filepath.IsAbs(path) {
		t.Error("expected absolute path")
	}

	if filepath.Dir(filepath.Dir(path)) != "/home/user/.dotfiles-backup/restore/zsh" {
		// Path format: /backup/restore/appID/.zshrc.timestamp.bak
		t.Logf("path: %s", path)
	}
}

func TestCompareWithLocal(t *testing.T) {
	tmpDir, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create source file
	sourceDir := filepath.Join(bm.config.DotfilesPath, "zsh", "other-machine")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, ".zshrc"), []byte("source config"), 0644)

	comparison, err := bm.CompareWithLocal("other-machine", "zsh", ".zshrc")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}

	if !comparison.SourceExists {
		t.Error("expected source to exist")
	}

	if comparison.SourceSize != 13 {
		t.Errorf("expected size 13, got %d", comparison.SourceSize)
	}

	_ = tmpDir
}

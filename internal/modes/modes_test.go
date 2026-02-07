package modes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.DefaultMode != ModeBackup {
		t.Errorf("expected default mode to be backup, got %s", cfg.DefaultMode)
	}

	if cfg.MachineName == "" {
		t.Error("expected machine name to be set from hostname")
	}

	if cfg.Apps == nil {
		t.Error("expected apps map to be initialized")
	}

	if cfg.Files == nil {
		t.Error("expected files map to be initialized")
	}
}

func TestModeToggle(t *testing.T) {
	if ModeSync.Toggle() != ModeBackup {
		t.Error("sync should toggle to backup")
	}

	if ModeBackup.Toggle() != ModeSync {
		t.Error("backup should toggle to sync")
	}
}

func TestModeShort(t *testing.T) {
	if ModeSync.Short() != "S" {
		t.Errorf("expected S, got %s", ModeSync.Short())
	}

	if ModeBackup.Short() != "B" {
		t.Errorf("expected B, got %s", ModeBackup.Short())
	}
}

func TestModeIsSync(t *testing.T) {
	if !ModeSync.IsSync() {
		t.Error("ModeSync.IsSync() should be true")
	}

	if ModeBackup.IsSync() {
		t.Error("ModeBackup.IsSync() should be false")
	}
}

func TestModeIsBackup(t *testing.T) {
	if !ModeBackup.IsBackup() {
		t.Error("ModeBackup.IsBackup() should be true")
	}

	if ModeSync.IsBackup() {
		t.Error("ModeSync.IsBackup() should be false")
	}
}

func TestLoadSave(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create config directory
	configDir := filepath.Join(tmpDir, ".config", "dotsync")
	os.MkdirAll(configDir, 0755)

	// First load should return defaults
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultMode != ModeBackup {
		t.Errorf("expected default mode backup, got %s", cfg.DefaultMode)
	}

	// Modify and save
	cfg.MachineName = "test-machine"
	cfg.SetAppMode("zsh", ModeSync)
	cfg.SetFileMode("zsh/.zshrc.local", ModeBackup)

	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Load again and verify
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if cfg2.MachineName != "test-machine" {
		t.Errorf("expected machine name test-machine, got %s", cfg2.MachineName)
	}

	if cfg2.Apps["zsh"] != ModeSync {
		t.Errorf("expected zsh app mode sync, got %s", cfg2.Apps["zsh"])
	}

	if cfg2.Files["zsh/.zshrc.local"] != ModeBackup {
		t.Errorf("expected file mode backup, got %s", cfg2.Files["zsh/.zshrc.local"])
	}
}

func TestSetAndRemoveModes(t *testing.T) {
	cfg := Default()

	// Set app mode
	cfg.SetAppMode("nvim", ModeSync)
	if cfg.Apps["nvim"] != ModeSync {
		t.Error("app mode not set correctly")
	}

	// Remove app mode
	cfg.RemoveAppMode("nvim")
	if _, ok := cfg.Apps["nvim"]; ok {
		t.Error("app mode should be removed")
	}

	// Set file mode
	cfg.SetFileMode("zsh/.zshrc", ModeSync)
	if cfg.Files["zsh/.zshrc"] != ModeSync {
		t.Error("file mode not set correctly")
	}

	// Remove file mode
	cfg.RemoveFileMode("zsh/.zshrc")
	if _, ok := cfg.Files["zsh/.zshrc"]; ok {
		t.Error("file mode should be removed")
	}
}

package modes

import (
	"testing"
)

func TestGetMode(t *testing.T) {
	cfg := &ModesConfig{
		MachineName: "test-machine",
		DefaultMode: ModeBackup,
		Apps: map[string]Mode{
			"zsh": ModeSync,
			"git": ModeSync,
		},
		Files: map[string]Mode{
			"zsh/.zshrc.local": ModeBackup,
		},
	}

	tests := []struct {
		name     string
		appID    string
		filePath string
		expected Mode
	}{
		{
			name:     "file override takes priority",
			appID:    "zsh",
			filePath: ".zshrc.local",
			expected: ModeBackup,
		},
		{
			name:     "app setting when no file override",
			appID:    "zsh",
			filePath: ".zshrc",
			expected: ModeSync,
		},
		{
			name:     "app setting for git",
			appID:    "git",
			filePath: ".gitconfig",
			expected: ModeSync,
		},
		{
			name:     "default mode when no overrides",
			appID:    "tmux",
			filePath: ".tmux.conf",
			expected: ModeBackup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := cfg.GetMode(tt.appID, tt.filePath)
			if mode != tt.expected {
				t.Errorf("GetMode(%s, %s) = %s, want %s", tt.appID, tt.filePath, mode, tt.expected)
			}
		})
	}
}

func TestGetAppMode(t *testing.T) {
	cfg := &ModesConfig{
		DefaultMode: ModeBackup,
		Apps: map[string]Mode{
			"zsh": ModeSync,
		},
		Files: make(map[string]Mode),
	}

	// App with override
	if cfg.GetAppMode("zsh") != ModeSync {
		t.Error("expected zsh to be sync")
	}

	// App without override
	if cfg.GetAppMode("unknown") != ModeBackup {
		t.Error("expected unknown app to use default mode")
	}
}

func TestHasOverrides(t *testing.T) {
	cfg := &ModesConfig{
		DefaultMode: ModeBackup,
		Apps: map[string]Mode{
			"zsh": ModeSync,
		},
		Files: map[string]Mode{
			"zsh/.zshrc": ModeSync,
		},
	}

	if !cfg.HasAppOverride("zsh") {
		t.Error("expected zsh to have app override")
	}

	if cfg.HasAppOverride("unknown") {
		t.Error("expected unknown to not have app override")
	}

	if !cfg.HasFileOverride("zsh", ".zshrc") {
		t.Error("expected .zshrc to have file override")
	}

	if cfg.HasFileOverride("zsh", ".zprofile") {
		t.Error("expected .zprofile to not have file override")
	}
}

func TestGetEffectiveMode(t *testing.T) {
	cfg := &ModesConfig{
		DefaultMode: ModeBackup,
		Apps: map[string]Mode{
			"zsh": ModeSync,
		},
		Files: map[string]Mode{
			"zsh/.zshrc.local": ModeBackup,
		},
	}

	// File override
	mode, source := cfg.GetEffectiveMode("zsh", ".zshrc.local")
	if mode != ModeBackup || source != "file" {
		t.Errorf("expected backup from file, got %s from %s", mode, source)
	}

	// App override
	mode, source = cfg.GetEffectiveMode("zsh", ".zshrc")
	if mode != ModeSync || source != "app" {
		t.Errorf("expected sync from app, got %s from %s", mode, source)
	}

	// Default
	mode, source = cfg.GetEffectiveMode("unknown", "file")
	if mode != ModeBackup || source != "default" {
		t.Errorf("expected backup from default, got %s from %s", mode, source)
	}
}

func TestToggleModes(t *testing.T) {
	cfg := &ModesConfig{
		DefaultMode: ModeBackup,
		Apps:        make(map[string]Mode),
		Files:       make(map[string]Mode),
	}

	// Toggle app mode (default is backup)
	newMode := cfg.ToggleAppMode("zsh")
	if newMode != ModeSync {
		t.Errorf("expected sync after toggle, got %s", newMode)
	}

	// Toggle again
	newMode = cfg.ToggleAppMode("zsh")
	if newMode != ModeBackup {
		t.Errorf("expected backup after second toggle, got %s", newMode)
	}

	// Toggle file mode
	newMode = cfg.ToggleFileMode("zsh", ".zshrc")
	if newMode != ModeSync {
		t.Errorf("expected sync after file toggle, got %s", newMode)
	}
}

func TestSetAllAppsMode(t *testing.T) {
	cfg := Default()

	appIDs := []string{"zsh", "git", "nvim"}
	cfg.SetAllAppsMode(appIDs, ModeSync)

	for _, appID := range appIDs {
		if cfg.Apps[appID] != ModeSync {
			t.Errorf("expected %s to be sync", appID)
		}
	}
}

func TestNormalizeFilePath(t *testing.T) {
	tests := []struct {
		appID    string
		filePath string
		expected string
	}{
		{"zsh", ".zshrc", "zsh/.zshrc"},
		{"zsh", "zsh/.zshrc", "zsh/.zshrc"},
		{"git", "/home/user/.gitconfig", "git/.gitconfig"},
	}

	for _, tt := range tests {
		result := normalizeFilePath(tt.appID, tt.filePath)
		if result != tt.expected {
			t.Errorf("normalizeFilePath(%s, %s) = %s, want %s", tt.appID, tt.filePath, result, tt.expected)
		}
	}
}

func TestStoragePaths(t *testing.T) {
	cfg := &ModesConfig{
		MachineName: "my-machine",
		DefaultMode: ModeBackup,
		Apps: map[string]Mode{
			"git": ModeSync,
		},
		Files: make(map[string]Mode),
	}

	basePath := "/home/user/dotfiles"

	// Backup path
	backupPath := cfg.GetBackupPath(basePath, "zsh", ".zshrc")
	expected := "/home/user/dotfiles/zsh/my-machine/.zshrc"
	if backupPath != expected {
		t.Errorf("GetBackupPath = %s, want %s", backupPath, expected)
	}

	// Sync path
	syncPath := cfg.GetSyncPath(basePath, "git", ".gitconfig")
	expected = "/home/user/dotfiles/git/.gitconfig"
	if syncPath != expected {
		t.Errorf("GetSyncPath = %s, want %s", syncPath, expected)
	}

	// GetStoragePath for backup mode app
	storagePath := cfg.GetStoragePath(basePath, "zsh", ".zshrc")
	expected = "/home/user/dotfiles/zsh/my-machine/.zshrc"
	if storagePath != expected {
		t.Errorf("GetStoragePath (backup) = %s, want %s", storagePath, expected)
	}

	// GetStoragePath for sync mode app
	storagePath = cfg.GetStoragePath(basePath, "git", ".gitconfig")
	expected = "/home/user/dotfiles/git/.gitconfig"
	if storagePath != expected {
		t.Errorf("GetStoragePath (sync) = %s, want %s", storagePath, expected)
	}
}

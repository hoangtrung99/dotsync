package modes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Version != 2 {
		t.Errorf("expected version 2, got %d", cfg.Version)
	}

	if cfg.MachineName == "" {
		t.Error("expected machine name to be set from hostname")
	}

	if cfg.SyncedApps == nil {
		t.Error("expected synced apps map to be initialized")
	}

	if cfg.SyncedFiles == nil {
		t.Error("expected synced files map to be initialized")
	}
}

func TestIsSynced(t *testing.T) {
	cfg := &ModesConfig{
		Version:     2,
		MachineName: "test",
		SyncedApps:  map[string]bool{"zsh": true},
		SyncedFiles: map[string]bool{"git/.gitignore": true},
	}

	// App-level sync
	if !cfg.IsSynced("zsh", ".zshrc") {
		t.Error("zsh/.zshrc should be synced (app-level)")
	}

	// File-level sync
	if !cfg.IsSynced("git", ".gitignore") {
		t.Error("git/.gitignore should be synced (file-level)")
	}

	// Not synced
	if cfg.IsSynced("tmux", ".tmux.conf") {
		t.Error("tmux should not be synced")
	}

	// App not synced, but file override not present
	if cfg.IsSynced("git", ".gitconfig") {
		t.Error("git/.gitconfig should not be synced (no file override, app not synced)")
	}
}

func TestIsAppSynced(t *testing.T) {
	cfg := &ModesConfig{
		Version:     2,
		MachineName: "test",
		SyncedApps:  map[string]bool{"zsh": true},
		SyncedFiles: make(map[string]bool),
	}

	if !cfg.IsAppSynced("zsh") {
		t.Error("zsh should be synced")
	}

	if cfg.IsAppSynced("unknown") {
		t.Error("unknown should not be synced")
	}
}

func TestToggleAppSync(t *testing.T) {
	cfg := Default()

	// Toggle on (default is off)
	synced := cfg.ToggleAppSync("zsh")
	if !synced {
		t.Error("expected sync ON after first toggle")
	}
	if !cfg.SyncedApps["zsh"] {
		t.Error("expected zsh in SyncedApps")
	}

	// Toggle off
	synced = cfg.ToggleAppSync("zsh")
	if synced {
		t.Error("expected sync OFF after second toggle")
	}
	if _, ok := cfg.SyncedApps["zsh"]; ok {
		t.Error("expected zsh removed from SyncedApps")
	}
}

func TestToggleFileSync(t *testing.T) {
	cfg := Default()

	synced := cfg.ToggleFileSync("zsh", ".zshrc")
	if !synced {
		t.Error("expected sync ON after toggle")
	}
	if !cfg.SyncedFiles["zsh/.zshrc"] {
		t.Error("expected zsh/.zshrc in SyncedFiles")
	}

	synced = cfg.ToggleFileSync("zsh", ".zshrc")
	if synced {
		t.Error("expected sync OFF after second toggle")
	}
}

func TestSyncLabel(t *testing.T) {
	cfg := &ModesConfig{
		Version:     2,
		MachineName: "test",
		SyncedApps:  map[string]bool{"zsh": true},
		SyncedFiles: make(map[string]bool),
	}

	if cfg.SyncLabel("zsh", ".zshrc") != "B+S" {
		t.Errorf("expected B+S, got %s", cfg.SyncLabel("zsh", ".zshrc"))
	}

	if cfg.SyncLabel("tmux", ".tmux.conf") != "B" {
		t.Errorf("expected B, got %s", cfg.SyncLabel("tmux", ".tmux.conf"))
	}
}

func TestAppSyncLabel(t *testing.T) {
	cfg := &ModesConfig{
		Version:     2,
		MachineName: "test",
		SyncedApps:  map[string]bool{"zsh": true},
		SyncedFiles: make(map[string]bool),
	}

	if cfg.AppSyncLabel("zsh") != "B+S" {
		t.Errorf("expected B+S, got %s", cfg.AppSyncLabel("zsh"))
	}

	if cfg.AppSyncLabel("tmux") != "B" {
		t.Errorf("expected B, got %s", cfg.AppSyncLabel("tmux"))
	}
}

func TestLoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	configDir := filepath.Join(tmpDir, ".config", "dotsync")
	os.MkdirAll(configDir, 0755)

	// First load should return defaults
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Version != 2 {
		t.Errorf("expected version 2, got %d", cfg.Version)
	}

	// Modify and save
	cfg.MachineName = "test-machine"
	cfg.SyncedApps["zsh"] = true
	cfg.SyncedFiles["git/.gitignore"] = true

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

	if !cfg2.SyncedApps["zsh"] {
		t.Error("expected zsh to be synced")
	}

	if !cfg2.SyncedFiles["git/.gitignore"] {
		t.Error("expected git/.gitignore to be synced")
	}
}

func TestMigrateV1(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	configDir := filepath.Join(tmpDir, ".config", "dotsync")
	os.MkdirAll(configDir, 0755)

	// Write v1 config
	v1Data := map[string]interface{}{
		"machine_name": "old-machine",
		"default_mode": "backup",
		"apps": map[string]string{
			"zsh": "sync",
			"git": "backup",
		},
		"files": map[string]string{
			"git/.gitignore": "sync",
			"zsh/.zshrc.local": "backup",
		},
	}

	data, _ := json.MarshalIndent(v1Data, "", "  ")
	configPath := filepath.Join(configDir, "modes.json")
	os.WriteFile(configPath, data, 0644)

	// Load should trigger migration
	cfg, err := Load()
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	if cfg.Version != 2 {
		t.Errorf("expected version 2 after migration, got %d", cfg.Version)
	}

	if cfg.MachineName != "old-machine" {
		t.Errorf("expected machine name old-machine, got %s", cfg.MachineName)
	}

	// zsh was "sync" → should be in SyncedApps
	if !cfg.SyncedApps["zsh"] {
		t.Error("expected zsh to be synced after migration")
	}

	// git was "backup" → should NOT be in SyncedApps
	if cfg.SyncedApps["git"] {
		t.Error("expected git to not be synced after migration")
	}

	// git/.gitignore was "sync" → should be in SyncedFiles
	if !cfg.SyncedFiles["git/.gitignore"] {
		t.Error("expected git/.gitignore to be synced after migration")
	}

	// zsh/.zshrc.local was "backup" → should NOT be in SyncedFiles
	if cfg.SyncedFiles["zsh/.zshrc.local"] {
		t.Error("expected zsh/.zshrc.local to not be synced after migration")
	}
}

func TestStoragePaths(t *testing.T) {
	cfg := &ModesConfig{
		Version:     2,
		MachineName: "my-machine",
		SyncedApps:  make(map[string]bool),
		SyncedFiles: make(map[string]bool),
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
}

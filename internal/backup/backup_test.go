package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dotsync/internal/config"
	"dotsync/internal/modes"
	"dotsync/internal/models"
)

func setupTestEnv(t *testing.T) (string, *BackupManager, func()) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		DotfilesPath: filepath.Join(tmpDir, "dotfiles"),
		BackupPath:   filepath.Join(tmpDir, "backup"),
	}

	modesCfg := &modes.ModesConfig{
		MachineName: "test-machine",
		DefaultMode: modes.ModeBackup,
		Apps:        make(map[string]modes.Mode),
		Files:       make(map[string]modes.Mode),
	}

	os.MkdirAll(cfg.DotfilesPath, 0755)
	os.MkdirAll(cfg.BackupPath, 0755)

	bm := New(cfg, modesCfg)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, bm, cleanup
}

func TestNew(t *testing.T) {
	cfg := &config.Config{}
	modesCfg := &modes.ModesConfig{}

	bm := New(cfg, modesCfg)

	if bm == nil {
		t.Fatal("expected non-nil BackupManager")
	}

	if bm.config != cfg {
		t.Error("config not set correctly")
	}

	if bm.modesConfig != modesCfg {
		t.Error("modesConfig not set correctly")
	}
}

func TestBackup(t *testing.T) {
	tmpDir, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a test file
	testFile := filepath.Join(tmpDir, ".zshrc")
	os.WriteFile(testFile, []byte("# test config"), 0644)

	apps := []*models.App{
		{
			ID:       "zsh",
			Name:     "Zsh",
			Selected: true,
			Files: []models.File{
				{
					Name:     ".zshrc",
					Path:     testFile,
					Selected: true,
					Size:     13,
				},
			},
		},
	}

	result, err := bm.Backup(apps)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	if len(result.BackedUp) != 1 {
		t.Errorf("expected 1 backed up file, got %d", len(result.BackedUp))
	}

	if len(result.Skipped) != 0 {
		t.Errorf("expected 0 skipped files, got %d", len(result.Skipped))
	}

	// Verify file was copied
	expectedDest := filepath.Join(bm.config.DotfilesPath, "zsh", "test-machine", ".zshrc")
	if _, err := os.Stat(expectedDest); err != nil {
		t.Errorf("backup file not found: %v", err)
	}
}

func TestBackupSkipsSyncMode(t *testing.T) {
	tmpDir, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	// Set zsh to sync mode
	bm.modesConfig.SetAppMode("zsh", modes.ModeSync)

	testFile := filepath.Join(tmpDir, ".zshrc")
	os.WriteFile(testFile, []byte("# test"), 0644)

	apps := []*models.App{
		{
			ID:       "zsh",
			Selected: true,
			Files: []models.File{
				{
					Name:     ".zshrc",
					Path:     testFile,
					Selected: true,
				},
			},
		},
	}

	result, err := bm.Backup(apps)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	if len(result.BackedUp) != 0 {
		t.Errorf("expected 0 backed up (sync mode), got %d", len(result.BackedUp))
	}

	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}

	if result.Skipped[0].Reason != "sync mode" {
		t.Errorf("expected 'sync mode' reason, got '%s'", result.Skipped[0].Reason)
	}
}

func TestListMachines(t *testing.T) {
	_, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	// Initially empty
	machines, err := bm.ListMachines()
	if err != nil {
		t.Fatalf("list machines failed: %v", err)
	}

	if len(machines) != 0 {
		t.Errorf("expected 0 machines initially, got %d", len(machines))
	}

	// Create machines.json
	mf := &MachinesFile{
		Machines: []Machine{
			{Name: "machine-a", LastSync: time.Now()},
			{Name: "machine-b", LastSync: time.Now().Add(-24 * time.Hour)},
		},
	}
	bm.saveMachinesFile(mf)

	machines, err = bm.ListMachines()
	if err != nil {
		t.Fatalf("list machines failed: %v", err)
	}

	if len(machines) != 2 {
		t.Errorf("expected 2 machines, got %d", len(machines))
	}
}

func TestUpdateMachinesFile(t *testing.T) {
	_, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	// First update - should create file
	err := bm.updateMachinesFile()
	if err != nil {
		t.Fatalf("update machines failed: %v", err)
	}

	machines, _ := bm.ListMachines()
	if len(machines) != 1 {
		t.Errorf("expected 1 machine, got %d", len(machines))
	}

	if machines[0].Name != "test-machine" {
		t.Errorf("expected 'test-machine', got '%s'", machines[0].Name)
	}

	// Update again - should update existing
	time.Sleep(10 * time.Millisecond)
	oldSync := machines[0].LastSync
	bm.updateMachinesFile()

	machines, _ = bm.ListMachines()
	if len(machines) != 1 {
		t.Errorf("expected still 1 machine after update, got %d", len(machines))
	}

	if !machines[0].LastSync.After(oldSync) {
		t.Error("expected LastSync to be updated")
	}
}

func TestHasBackups(t *testing.T) {
	_, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	if bm.HasBackups() {
		t.Error("expected no backups initially")
	}

	bm.updateMachinesFile()

	if !bm.HasBackups() {
		t.Error("expected to have backups after update")
	}
}

func TestGetBackupStats(t *testing.T) {
	_, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	bm.updateMachinesFile()

	count, lastSync, err := bm.GetBackupStats()
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 machine, got %d", count)
	}

	if lastSync.IsZero() {
		t.Error("expected non-zero lastSync")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir, bm, cleanup := setupTestEnv(t)
	defer cleanup()

	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "subdir", "dest.txt")

	content := []byte("test content")
	os.WriteFile(src, content, 0644)

	err := bm.copyFile(src, dst)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	// Verify
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("content mismatch: got '%s', want '%s'", data, content)
	}
}

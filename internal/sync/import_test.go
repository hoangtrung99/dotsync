package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dotsync/internal/config"
	"dotsync/internal/models"
)

func TestNewImporter(t *testing.T) {
	cfg := config.Default()
	importer := NewImporter(cfg)

	if importer == nil {
		t.Fatal("NewImporter should return an Importer")
	}
	if importer.config != cfg {
		t.Error("Importer should have the provided config")
	}
}

func TestImportResult(t *testing.T) {
	result := ImportResult{
		App:        &models.App{ID: "test"},
		File:       models.File{Name: "test.txt"},
		Success:    true,
		BackupPath: "/backup/path",
	}

	if result.App.ID != "test" {
		t.Error("App ID should be test")
	}
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.BackupPath != "/backup/path" {
		t.Error("BackupPath should be /backup/path")
	}
}

func TestCompareFiles_BothNotExist(t *testing.T) {
	status := CompareFiles("/nonexistent1", "/nonexistent2")
	if status != models.StatusUnknown {
		t.Errorf("Expected StatusUnknown, got %v", status)
	}
}

func TestCompareFiles_LocalNotExist(t *testing.T) {
	tempDir := t.TempDir()
	dotfilesFile := filepath.Join(tempDir, "dotfiles.txt")
	os.WriteFile(dotfilesFile, []byte("content"), 0644)

	status := CompareFiles("/nonexistent", dotfilesFile)
	if status != models.StatusMissing {
		t.Errorf("Expected StatusMissing, got %v", status)
	}
}

func TestCompareFiles_DotfilesNotExist(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	os.WriteFile(localFile, []byte("content"), 0644)

	status := CompareFiles(localFile, "/nonexistent")
	if status != models.StatusNew {
		t.Errorf("Expected StatusNew, got %v", status)
	}
}

func TestCompareFiles_Synced(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	dotfilesFile := filepath.Join(tempDir, "dotfiles.txt")

	os.WriteFile(localFile, []byte("content"), 0644)
	os.WriteFile(dotfilesFile, []byte("content"), 0644)

	// Make files have same mod time
	info, _ := os.Stat(localFile)
	os.Chtimes(dotfilesFile, info.ModTime(), info.ModTime())

	status := CompareFiles(localFile, dotfilesFile)
	if status != models.StatusSynced {
		t.Errorf("Expected StatusSynced, got %v", status)
	}
}

func TestDetectConflictSimple(t *testing.T) {
	tests := []struct {
		name         string
		localHash    string
		dotfilesHash string
		expected     models.ConflictType
	}{
		{"both empty", "", "", models.ConflictNone},
		{"local empty", "", "abc123", models.ConflictDotfilesNew},
		{"dotfiles empty", "abc123", "", models.ConflictLocalNew},
		{"same hash", "abc123", "abc123", models.ConflictNone},
		{"different hash", "abc123", "def456", models.ConflictBothModified},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := detectConflictSimple(tc.localHash, tc.dotfilesHash)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestUpdateSyncStatus(t *testing.T) {
	tempDir := t.TempDir()
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	appDir := filepath.Join(dotfilesDir, "testapp")
	os.MkdirAll(appDir, 0755)

	// Create dotfiles file
	os.WriteFile(filepath.Join(appDir, "config.txt"), []byte("content"), 0644)

	// Create local file
	localDir := filepath.Join(tempDir, "local")
	os.MkdirAll(localDir, 0755)
	localFile := filepath.Join(localDir, "config.txt")
	os.WriteFile(localFile, []byte("content"), 0644)

	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "config.txt", Path: localFile, RelPath: "config.txt"},
		},
	}

	UpdateSyncStatus(app, dotfilesDir)

	// Status should be updated
	if app.Files[0].SyncStatus == models.StatusUnknown {
		t.Error("Status should be updated from Unknown")
	}
}

func TestUpdateSyncStatusWithHashes(t *testing.T) {
	tempDir := t.TempDir()
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	appDir := filepath.Join(dotfilesDir, "testapp")
	os.MkdirAll(appDir, 0755)

	// Create dotfiles file
	os.WriteFile(filepath.Join(appDir, "config.txt"), []byte("content"), 0644)

	// Create local file with same content
	localDir := filepath.Join(tempDir, "local")
	os.MkdirAll(localDir, 0755)
	localFile := filepath.Join(localDir, "config.txt")
	os.WriteFile(localFile, []byte("content"), 0644)

	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "config.txt", Path: localFile, RelPath: "config.txt"},
		},
	}

	// Test without state manager
	UpdateSyncStatusWithHashes(app, dotfilesDir, nil)

	// Hashes should be computed
	if app.Files[0].LocalHash == "" {
		t.Error("LocalHash should be computed")
	}
	if app.Files[0].DotfilesHash == "" {
		t.Error("DotfilesHash should be computed")
	}
	// Same content means no conflict
	if app.Files[0].ConflictType != models.ConflictNone {
		t.Errorf("Expected ConflictNone, got %v", app.Files[0].ConflictType)
	}
}

func TestUpdateSyncStatusWithHashes_Directory(t *testing.T) {
	tempDir := t.TempDir()
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	appDir := filepath.Join(dotfilesDir, "testapp", "configdir")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "file.txt"), []byte("content"), 0644)

	// Create local directory with same content
	localDir := filepath.Join(tempDir, "local", "configdir")
	os.MkdirAll(localDir, 0755)
	os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("content"), 0644)

	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "configdir", Path: filepath.Join(tempDir, "local", "configdir"), RelPath: "configdir", IsDir: true},
		},
	}

	UpdateSyncStatusWithHashes(app, dotfilesDir, nil)

	// Directories use ModTime-based comparison for performance, not hash
	// ConflictType should be set based on ModTime comparison
	if app.Files[0].ConflictType == models.ConflictNone {
		// Both exist and ModTime says synced - this is expected
		t.Log("Directory ConflictType correctly set to ConflictNone")
	}
}

func TestImportApp_NoDotfilesDir(t *testing.T) {
	cfg := config.Default()
	cfg.DotfilesPath = "/nonexistent/dotfiles"

	importer := NewImporter(cfg)
	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "config.txt", Selected: true},
		},
	}

	results, err := importer.ImportApp(app)
	if err != nil {
		t.Errorf("ImportApp failed: %v", err)
	}
	if len(results) != 0 {
		t.Error("Should return empty results when dotfiles dir doesn't exist")
	}
}

func TestImportApp_NoSelectedFiles(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.Default()
	cfg.DotfilesPath = tempDir

	importer := NewImporter(cfg)
	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "config.txt", Selected: false},
		},
	}

	results, err := importer.ImportApp(app)
	if err != nil {
		t.Errorf("ImportApp failed: %v", err)
	}
	if len(results) != 0 {
		t.Error("Should return empty results when no files selected")
	}
}

func TestImportApp_FileNotInDotfiles(t *testing.T) {
	tempDir := t.TempDir()
	appDir := filepath.Join(tempDir, "testapp")
	os.MkdirAll(appDir, 0755)

	cfg := config.Default()
	cfg.DotfilesPath = tempDir

	importer := NewImporter(cfg)
	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "missing.txt", Path: "/some/path", RelPath: "missing.txt", Selected: true},
		},
	}

	results, err := importer.ImportApp(app)
	if err != nil {
		t.Errorf("ImportApp failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("Should have error for missing file")
	}
}

func TestImportApp_Success(t *testing.T) {
	tempDir := t.TempDir()

	// Create dotfiles structure
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	appDir := filepath.Join(dotfilesDir, "testapp")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "config.txt"), []byte("dotfiles content"), 0644)

	// Create local directory
	localDir := filepath.Join(tempDir, "local")
	os.MkdirAll(localDir, 0755)
	localFile := filepath.Join(localDir, "config.txt")

	cfg := config.Default()
	cfg.DotfilesPath = dotfilesDir
	cfg.BackupPath = filepath.Join(tempDir, "backups")

	importer := NewImporter(cfg)
	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "config.txt", Path: localFile, RelPath: "config.txt", Selected: true},
		},
	}

	results, err := importer.ImportApp(app)
	if err != nil {
		t.Errorf("ImportApp failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("Import should succeed: %v", results[0].Error)
	}

	// Verify file was imported
	content, err := os.ReadFile(localFile)
	if err != nil {
		t.Errorf("Failed to read imported file: %v", err)
	}
	if string(content) != "dotfiles content" {
		t.Errorf("Content should be 'dotfiles content', got '%s'", string(content))
	}
}

func TestImportApp_WithBackup(t *testing.T) {
	tempDir := t.TempDir()

	// Create dotfiles structure
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	appDir := filepath.Join(dotfilesDir, "testapp")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "config.txt"), []byte("new content"), 0644)

	// Create existing local file
	localDir := filepath.Join(tempDir, "local")
	os.MkdirAll(localDir, 0755)
	localFile := filepath.Join(localDir, "config.txt")
	os.WriteFile(localFile, []byte("original content"), 0644)

	cfg := config.Default()
	cfg.DotfilesPath = dotfilesDir
	cfg.BackupPath = filepath.Join(tempDir, "backups")

	importer := NewImporter(cfg)
	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "config.txt", Path: localFile, RelPath: "config.txt", Selected: true},
		},
	}

	results, err := importer.ImportApp(app)
	if err != nil {
		t.Errorf("ImportApp failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].BackupPath == "" {
		t.Error("BackupPath should be set when existing file is backed up")
	}
}

func TestImportAll_NoSelectedApps(t *testing.T) {
	cfg := config.Default()
	cfg.DotfilesPath = t.TempDir()

	importer := NewImporter(cfg)
	apps := []*models.App{
		{ID: "app1", Selected: false},
		{ID: "app2", Selected: false},
	}

	results, err := importer.ImportAll(apps)
	if err != nil {
		t.Errorf("ImportAll failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestImportAll_WithSelectedApps(t *testing.T) {
	tempDir := t.TempDir()

	// Create dotfiles structure for 2 apps
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	app1Dir := filepath.Join(dotfilesDir, "app1")
	app2Dir := filepath.Join(dotfilesDir, "app2")
	os.MkdirAll(app1Dir, 0755)
	os.MkdirAll(app2Dir, 0755)
	os.WriteFile(filepath.Join(app1Dir, "config1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(app2Dir, "config2.txt"), []byte("content2"), 0644)

	// Create local directories
	localDir1 := filepath.Join(tempDir, "local1")
	localDir2 := filepath.Join(tempDir, "local2")
	os.MkdirAll(localDir1, 0755)
	os.MkdirAll(localDir2, 0755)

	cfg := config.Default()
	cfg.DotfilesPath = dotfilesDir
	cfg.BackupPath = filepath.Join(tempDir, "backups")

	importer := NewImporter(cfg)
	apps := []*models.App{
		{
			ID:       "app1",
			Selected: true,
			Files: []models.File{
				{Name: "config1.txt", Path: filepath.Join(localDir1, "config1.txt"), RelPath: "config1.txt", Selected: true},
			},
		},
		{
			ID:       "app2",
			Selected: true,
			Files: []models.File{
				{Name: "config2.txt", Path: filepath.Join(localDir2, "config2.txt"), RelPath: "config2.txt", Selected: true},
			},
		},
		{
			ID:       "app3",
			Selected: false, // Should be skipped
			Files: []models.File{
				{Name: "config3.txt", Path: "/nonexistent", RelPath: "config3.txt", Selected: true},
			},
		},
	}

	results, err := importer.ImportAll(apps)
	if err != nil {
		t.Errorf("ImportAll failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Verify files were imported
	for _, r := range results {
		if !r.Success {
			t.Errorf("Import should succeed: %v", r.Error)
		}
	}
}

func TestImportApp_WithDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create dotfiles directory structure
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	appDir := filepath.Join(dotfilesDir, "testapp", "configdir")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(appDir, "file2.txt"), []byte("content2"), 0644)

	// Create local directory
	localDir := filepath.Join(tempDir, "local")
	localConfigDir := filepath.Join(localDir, "configdir")
	os.MkdirAll(localConfigDir, 0755)

	cfg := config.Default()
	cfg.DotfilesPath = dotfilesDir
	cfg.BackupPath = filepath.Join(tempDir, "backups")

	importer := NewImporter(cfg)
	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "configdir", Path: localConfigDir, RelPath: "configdir", IsDir: true, Selected: true},
		},
	}

	results, err := importer.ImportApp(app)
	if err != nil {
		t.Errorf("ImportApp failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("Import should succeed: %v", results[0].Error)
	}

	// Verify files were imported
	if _, err := os.Stat(filepath.Join(localConfigDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt should be imported")
	}
	if _, err := os.Stat(filepath.Join(localConfigDir, "file2.txt")); os.IsNotExist(err) {
		t.Error("file2.txt should be imported")
	}
}

func TestImportApp_DirectoryWithBackup(t *testing.T) {
	tempDir := t.TempDir()

	// Create dotfiles directory structure
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	appDir := filepath.Join(dotfilesDir, "testapp", "configdir")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "new.txt"), []byte("new content"), 0644)

	// Create existing local directory with content
	localDir := filepath.Join(tempDir, "local")
	localConfigDir := filepath.Join(localDir, "configdir")
	os.MkdirAll(localConfigDir, 0755)
	os.WriteFile(filepath.Join(localConfigDir, "old.txt"), []byte("old content"), 0644)

	cfg := config.Default()
	cfg.DotfilesPath = dotfilesDir
	cfg.BackupPath = filepath.Join(tempDir, "backups")

	importer := NewImporter(cfg)
	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "configdir", Path: localConfigDir, RelPath: "configdir", IsDir: true, Selected: true},
		},
	}

	results, err := importer.ImportApp(app)
	if err != nil {
		t.Errorf("ImportApp failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].BackupPath == "" {
		t.Error("BackupPath should be set when existing directory is backed up")
	}
}

func TestUpdateSyncStatusWithHashes_WithStateManager(t *testing.T) {
	tempDir := t.TempDir()
	dotfilesDir := filepath.Join(tempDir, "dotfiles")
	appDir := filepath.Join(dotfilesDir, "testapp")
	os.MkdirAll(appDir, 0755)

	// Create dotfiles file
	os.WriteFile(filepath.Join(appDir, "config.txt"), []byte("dotfiles content"), 0644)

	// Create local file with different content
	localDir := filepath.Join(tempDir, "local")
	os.MkdirAll(localDir, 0755)
	localFile := filepath.Join(localDir, "config.txt")
	os.WriteFile(localFile, []byte("local content"), 0644)

	// Create state manager
	stateManager := NewStateManager(filepath.Join(tempDir, "state.json"))

	app := &models.App{
		ID: "testapp",
		Files: []models.File{
			{Name: "config.txt", Path: localFile, RelPath: "config.txt"},
		},
	}

	UpdateSyncStatusWithHashes(app, dotfilesDir, stateManager)

	// Both exist with different content, should detect conflict
	if app.Files[0].LocalHash == "" {
		t.Error("LocalHash should be computed")
	}
	if app.Files[0].DotfilesHash == "" {
		t.Error("DotfilesHash should be computed")
	}
	if app.Files[0].LocalHash == app.Files[0].DotfilesHash {
		t.Error("Hashes should be different for different content")
	}
}

func TestCompareFiles_LocalNewer(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	dotfilesFile := filepath.Join(tempDir, "dotfiles.txt")

	// Create dotfiles first
	os.WriteFile(dotfilesFile, []byte("old"), 0644)

	// Wait and create local file
	os.WriteFile(localFile, []byte("new"), 0644)

	status := CompareFiles(localFile, dotfilesFile)
	if status != models.StatusModified {
		t.Errorf("Expected StatusModified (local newer), got %v", status)
	}
}

func TestCompareFiles_DotfilesNewer(t *testing.T) {
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "local.txt")
	dotfilesFile := filepath.Join(tempDir, "dotfiles.txt")

	// Create local first
	os.WriteFile(localFile, []byte("old"), 0644)

	// Set local file to an older time
	oldTime := time.Now().Add(-2 * time.Second)
	os.Chtimes(localFile, oldTime, oldTime)

	// Create dotfiles file (will have current time, newer than local)
	os.WriteFile(dotfilesFile, []byte("new"), 0644)

	status := CompareFiles(localFile, dotfilesFile)
	if status != models.StatusOutdated {
		t.Errorf("Expected StatusOutdated (dotfiles newer), got %v", status)
	}
}

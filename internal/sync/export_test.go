package sync

import (
	"os"
	"path/filepath"
	"testing"

	"dotsync/internal/config"
	"dotsync/internal/models"
)

func TestNewExporter(t *testing.T) {
	cfg := config.Default()
	exporter := NewExporter(cfg)

	if exporter == nil {
		t.Fatal("NewExporter should return an Exporter")
	}
	if exporter.config != cfg {
		t.Error("Exporter should have the provided config")
	}
}

func TestExportResult(t *testing.T) {
	result := ExportResult{
		App:       &models.App{ID: "test"},
		File:      models.File{Name: "test.txt"},
		Success:   true,
		Encrypted: false,
	}

	if result.App.ID != "test" {
		t.Error("App ID should be test")
	}
	if !result.Success {
		t.Error("Success should be true")
	}
}

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{".DS_Store", true},
		{".git", true},
		{"node_modules", true},
		{"__pycache__", true},
		{".cache", true},
		{"Cache", true},
		{"normal_file.txt", false},
		{"config.json", false},
	}

	for _, tc := range tests {
		result := shouldSkipFile(tc.name)
		if result != tc.expected {
			t.Errorf("shouldSkipFile(%s) = %v, expected %v", tc.name, result, tc.expected)
		}
	}
}

func TestBackup_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	backupPath, err := Backup("/nonexistent/path", tempDir)

	if err != nil {
		t.Errorf("Backup should not error for non-existent file: %v", err)
	}
	if backupPath != "" {
		t.Error("Backup path should be empty for non-existent file")
	}
}

func TestBackup_File(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	backupDir := filepath.Join(tempDir, "backups")
	backupPath, err := Backup(testFile, backupDir)

	if err != nil {
		t.Errorf("Backup failed: %v", err)
	}
	if backupPath == "" {
		t.Error("Backup path should not be empty")
	}

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Backup file should exist")
	}
}

func TestBackup_Directory(t *testing.T) {
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "testdir")
	os.MkdirAll(testDir, 0755)
	os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("content"), 0644)

	backupDir := filepath.Join(tempDir, "backups")
	backupPath, err := Backup(testDir, backupDir)

	if err != nil {
		t.Errorf("Backup failed: %v", err)
	}
	if backupPath == "" {
		t.Error("Backup path should not be empty")
	}

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Backup directory should exist")
	}
}

func TestExportApp_NoSelectedFiles(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.Default()
	cfg.DotfilesPath = tempDir

	exporter := NewExporter(cfg)
	app := &models.App{
		ID: "test",
		Files: []models.File{
			{Name: "file1.txt", Selected: false},
			{Name: "file2.txt", Selected: false},
		},
	}

	results, err := exporter.ExportApp(app)
	if err != nil {
		t.Errorf("ExportApp failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestExportApp_WithFiles(t *testing.T) {
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	dstDir := filepath.Join(tempDir, "dotfiles")
	os.MkdirAll(srcDir, 0755)

	// Create source file
	srcFile := filepath.Join(srcDir, "config.txt")
	os.WriteFile(srcFile, []byte("config content"), 0644)

	cfg := config.Default()
	cfg.DotfilesPath = dstDir

	exporter := NewExporter(cfg)
	app := &models.App{
		ID: "test",
		Files: []models.File{
			{Name: "config.txt", Path: srcFile, RelPath: "config.txt", Selected: true},
		},
	}

	results, err := exporter.ExportApp(app)
	if err != nil {
		t.Errorf("ExportApp failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("Export should succeed: %v", results[0].Error)
	}
}

func TestExportApp_WithDirectory(t *testing.T) {
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src", "configdir")
	dstDir := filepath.Join(tempDir, "dotfiles")
	os.MkdirAll(srcDir, 0755)

	// Create files in source directory
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644)

	cfg := config.Default()
	cfg.DotfilesPath = dstDir

	exporter := NewExporter(cfg)
	app := &models.App{
		ID: "test",
		Files: []models.File{
			{Name: "configdir", Path: srcDir, RelPath: "configdir", IsDir: true, Selected: true},
		},
	}

	results, err := exporter.ExportApp(app)
	if err != nil {
		t.Errorf("ExportApp failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("Export should succeed: %v", results[0].Error)
	}
}

func TestExportAll_NoSelectedApps(t *testing.T) {
	cfg := config.Default()
	cfg.DotfilesPath = t.TempDir()

	exporter := NewExporter(cfg)
	apps := []*models.App{
		{ID: "app1", Selected: false},
		{ID: "app2", Selected: false},
	}

	results, err := exporter.ExportAll(apps)
	if err != nil {
		t.Errorf("ExportAll failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestExportApp_SourceNotExist(t *testing.T) {
	cfg := config.Default()
	cfg.DotfilesPath = t.TempDir()

	exporter := NewExporter(cfg)
	app := &models.App{
		ID: "test",
		Files: []models.File{
			{Name: "missing.txt", Path: "/nonexistent/file.txt", RelPath: "missing.txt", Selected: true},
		},
	}

	results, err := exporter.ExportApp(app)
	if err != nil {
		t.Errorf("ExportApp failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Error("Export should fail for non-existent source")
	}
}

func TestCopyDir_SkipFiles(t *testing.T) {
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	dstDir := filepath.Join(tempDir, "dst")
	os.MkdirAll(srcDir, 0755)

	// Create files including ones that should be skipped
	os.WriteFile(filepath.Join(srcDir, "normal.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(srcDir, ".DS_Store"), []byte("skip"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "node_modules"), 0755)

	exporter := &Exporter{}
	err := exporter.copyDir(srcDir, dstDir)
	if err != nil {
		t.Errorf("copyDir failed: %v", err)
	}

	// Verify normal.txt was copied
	if _, err := os.Stat(filepath.Join(dstDir, "normal.txt")); os.IsNotExist(err) {
		t.Error("normal.txt should be copied")
	}

	// Verify .DS_Store was skipped
	if _, err := os.Stat(filepath.Join(dstDir, ".DS_Store")); !os.IsNotExist(err) {
		t.Error(".DS_Store should be skipped")
	}

	// Verify node_modules was skipped
	if _, err := os.Stat(filepath.Join(dstDir, "node_modules")); !os.IsNotExist(err) {
		t.Error("node_modules should be skipped")
	}
}

func TestExportAll_WithSelectedApps(t *testing.T) {
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	dstDir := filepath.Join(tempDir, "dotfiles")
	os.MkdirAll(srcDir, 0755)

	// Create source files
	srcFile1 := filepath.Join(srcDir, "config1.txt")
	srcFile2 := filepath.Join(srcDir, "config2.txt")
	os.WriteFile(srcFile1, []byte("content1"), 0644)
	os.WriteFile(srcFile2, []byte("content2"), 0644)

	cfg := config.Default()
	cfg.DotfilesPath = dstDir

	exporter := NewExporter(cfg)
	apps := []*models.App{
		{
			ID:       "app1",
			Selected: true,
			Files: []models.File{
				{Name: "config1.txt", Path: srcFile1, RelPath: "config1.txt", Selected: true},
			},
		},
		{
			ID:       "app2",
			Selected: true,
			Files: []models.File{
				{Name: "config2.txt", Path: srcFile2, RelPath: "config2.txt", Selected: true},
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

	results, err := exporter.ExportAll(apps)
	if err != nil {
		t.Errorf("ExportAll failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestCopyDir_WithSubdirectories(t *testing.T) {
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	dstDir := filepath.Join(tempDir, "dst")

	// Create nested directory structure
	os.MkdirAll(filepath.Join(srcDir, "subdir1", "nested"), 0755)
	os.MkdirAll(filepath.Join(srcDir, "subdir2"), 0755)
	os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir1", "file1.txt"), []byte("file1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir1", "nested", "deep.txt"), []byte("deep"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir2", "file2.txt"), []byte("file2"), 0644)

	exporter := &Exporter{}
	err := exporter.copyDir(srcDir, dstDir)
	if err != nil {
		t.Errorf("copyDir failed: %v", err)
	}

	// Verify all files were copied
	files := []string{
		"root.txt",
		"subdir1/file1.txt",
		"subdir1/nested/deep.txt",
		"subdir2/file2.txt",
	}
	for _, file := range files {
		path := filepath.Join(dstDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("File %s should be copied", file)
		}
	}
}

func TestCopyFile_Success(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "dest", "copied.txt")

	os.WriteFile(srcFile, []byte("test content"), 0644)

	exporter := &Exporter{}
	err := exporter.copyFile(srcFile, dstFile)
	if err != nil {
		t.Errorf("copyFile failed: %v", err)
	}

	// Verify content was copied
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Errorf("Failed to read destination: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("Content mismatch: got %s", string(content))
	}
}

func TestCopyFile_SourceNotExist(t *testing.T) {
	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "dest.txt")

	exporter := &Exporter{}
	err := exporter.copyFile("/nonexistent/file.txt", dstFile)
	if err == nil {
		t.Error("copyFile should fail for non-existent source")
	}
}

func TestShouldSkipFile_AllPatterns(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		// Skip patterns (exact match)
		{".DS_Store", true},
		{".git", true},
		{"node_modules", true},
		{"__pycache__", true},
		{".cache", true},
		{"Cache", true},
		// Non-skip patterns
		{"config.json", false},
		{"init.lua", false},
		{".gitconfig", false},
		{".gitignore", false},
		{"my_cache_file.txt", false},
		{"cache", false}, // lowercase cache doesn't match
		{"CACHE", false}, // uppercase CACHE doesn't match
	}

	for _, tc := range tests {
		result := shouldSkipFile(tc.name)
		if result != tc.expected {
			t.Errorf("shouldSkipFile(%s) = %v, expected %v", tc.name, result, tc.expected)
		}
	}
}

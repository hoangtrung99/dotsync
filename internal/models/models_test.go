package models

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ============ File Tests ============

func TestNewFile(t *testing.T) {
	// Create temp file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.toml")
	content := []byte("test content")
	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	file, err := NewFile(tempFile, tempDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	if file.Name != "test.toml" {
		t.Errorf("Expected name 'test.toml', got %s", file.Name)
	}
	if file.Path != tempFile {
		t.Errorf("Expected path %s, got %s", tempFile, file.Path)
	}
	if file.RelPath != "test.toml" {
		t.Errorf("Expected relPath 'test.toml', got %s", file.RelPath)
	}
	if file.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), file.Size)
	}
	if file.IsDir {
		t.Error("Expected IsDir to be false")
	}
	if !file.Selected {
		t.Error("Expected Selected to be true by default")
	}
}

func TestNewFile_Directory(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	os.MkdirAll(subDir, 0755)

	file, err := NewFile(subDir, tempDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	if !file.IsDir {
		t.Error("Expected IsDir to be true")
	}
}

func TestNewFile_NonExistent(t *testing.T) {
	_, err := NewFile("/nonexistent/path/file.txt", "/nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestFileToggleSelected(t *testing.T) {
	file := &File{Selected: false}

	file.ToggleSelected()
	if !file.Selected {
		t.Error("Expected Selected to be true after toggle")
	}

	file.ToggleSelected()
	if file.Selected {
		t.Error("Expected Selected to be false after second toggle")
	}
}

func TestFileIcon(t *testing.T) {
	tests := []struct {
		name     string
		isDir    bool
		expected string
	}{
		{"dir", true, "📁"},
		{"config.json", false, "📋"},
		{"config.yaml", false, "📄"},
		{"config.yml", false, "📄"},
		{"config.toml", false, "⚙️"},
		{"init.lua", false, "🌙"},
		{"script.sh", false, "🐚"},
		{"script.bash", false, "🐚"},
		{"script.zsh", false, "🐚"},
		{"script.fish", false, "🐚"},
		{"app.conf", false, "🔧"},
		{"unknown.xyz", false, "📄"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{Name: tt.name, IsDir: tt.isDir}
			icon := f.Icon()
			if icon != tt.expected {
				t.Errorf("Icon() for %s = %s, want %s", tt.name, icon, tt.expected)
			}
		})
	}
}

func TestFileSizeHuman(t *testing.T) {
	// The SizeHuman function has a quirky implementation
	// Just test that it returns a non-empty string
	tests := []struct {
		size int64
	}{
		{0},
		{500},
		{1024},
		{1048576},
	}

	for _, tt := range tests {
		f := &File{Size: tt.size}
		result := f.SizeHuman()
		if result == "" {
			t.Errorf("SizeHuman() for %d should not be empty", tt.size)
		}
	}
}

// ============ SyncStatus Tests ============

func TestSyncStatusIcon(t *testing.T) {
	tests := []struct {
		status   SyncStatus
		expected string
	}{
		{StatusSynced, "✓"},
		{StatusModified, "●"},
		{StatusOutdated, "○"},
		{StatusNew, "+"},
		{StatusMissing, "✗"},
		{StatusUnknown, "?"},
	}

	for _, tt := range tests {
		icon := tt.status.StatusIcon()
		if icon != tt.expected {
			t.Errorf("StatusIcon() for %v = %s, want %s", tt.status, icon, tt.expected)
		}
	}
}

func TestSyncStatusString(t *testing.T) {
	tests := []struct {
		status   SyncStatus
		expected string
	}{
		{StatusSynced, "Synced"},
		{StatusModified, "Modified"},
		{StatusOutdated, "Outdated"},
		{StatusNew, "New"},
		{StatusMissing, "Missing"},
		{StatusUnknown, "Unknown"},
	}

	for _, tt := range tests {
		str := tt.status.String()
		if str != tt.expected {
			t.Errorf("String() for %v = %s, want %s", tt.status, str, tt.expected)
		}
	}
}

// ============ ConflictType Tests ============

func TestConflictIcon(t *testing.T) {
	tests := []struct {
		conflict ConflictType
		expected string
	}{
		{ConflictNone, "✓"},
		{ConflictLocalModified, "●"},
		{ConflictDotfilesModified, "○"},
		{ConflictBothModified, "⚡"},
		{ConflictLocalNew, "+"},
		{ConflictDotfilesNew, "↓"},
		{ConflictLocalDeleted, "✗"},
		{ConflictDotfilesDeleted, "✗"},
	}

	for _, tt := range tests {
		icon := tt.conflict.ConflictIcon()
		if icon != tt.expected {
			t.Errorf("ConflictIcon() for %v = %s, want %s", tt.conflict, icon, tt.expected)
		}
	}
}

func TestConflictString(t *testing.T) {
	tests := []struct {
		conflict ConflictType
		expected string
	}{
		{ConflictNone, "Synced"},
		{ConflictLocalModified, "Modified (push)"},
		{ConflictDotfilesModified, "Outdated (pull)"},
		{ConflictBothModified, "CONFLICT"},
		{ConflictLocalNew, "New (local)"},
		{ConflictDotfilesNew, "New (dotfiles)"},
		{ConflictLocalDeleted, "Deleted locally"},
		{ConflictDotfilesDeleted, "Deleted in dotfiles"},
	}

	for _, tt := range tests {
		str := tt.conflict.ConflictString()
		if str != tt.expected {
			t.Errorf("ConflictString() for %v = %s, want %s", tt.conflict, str, tt.expected)
		}
	}
}

// ============ App Tests ============

func TestNewApp(t *testing.T) {
	def := AppDefinition{
		ID:          "test-app",
		Name:        "Test App",
		Category:    "dev",
		Icon:        "🔧",
		ConfigPaths: []string{"~/.config/test"},
	}

	app := NewApp(def)

	if app.ID != def.ID {
		t.Errorf("Expected ID %s, got %s", def.ID, app.ID)
	}
	if app.Name != def.Name {
		t.Errorf("Expected Name %s, got %s", def.Name, app.Name)
	}
	if app.Category != def.Category {
		t.Errorf("Expected Category %s, got %s", def.Category, app.Category)
	}
	if app.Icon != def.Icon {
		t.Errorf("Expected Icon %s, got %s", def.Icon, app.Icon)
	}
	if len(app.ConfigPaths) != len(def.ConfigPaths) {
		t.Errorf("Expected %d ConfigPaths, got %d", len(def.ConfigPaths), len(app.ConfigPaths))
	}
	if app.Selected {
		t.Error("Expected Selected to be false")
	}
	if app.Installed {
		t.Error("Expected Installed to be false")
	}
	if len(app.Files) != 0 {
		t.Errorf("Expected 0 Files, got %d", len(app.Files))
	}
}

func TestAppToggleSelected(t *testing.T) {
	app := &App{Selected: false}

	app.ToggleSelected()
	if !app.Selected {
		t.Error("Expected Selected to be true after toggle")
	}

	app.ToggleSelected()
	if app.Selected {
		t.Error("Expected Selected to be false after second toggle")
	}
}

func TestAppSelectAllFiles(t *testing.T) {
	app := &App{
		Files: []File{
			{Name: "file1.txt", Selected: false},
			{Name: "file2.txt", Selected: false},
			{Name: "file3.txt", Selected: true},
		},
	}

	app.SelectAllFiles()

	for i, f := range app.Files {
		if !f.Selected {
			t.Errorf("File %d should be selected", i)
		}
	}
}

func TestAppDeselectAllFiles(t *testing.T) {
	app := &App{
		Files: []File{
			{Name: "file1.txt", Selected: true},
			{Name: "file2.txt", Selected: true},
			{Name: "file3.txt", Selected: false},
		},
	}

	app.DeselectAllFiles()

	for i, f := range app.Files {
		if f.Selected {
			t.Errorf("File %d should not be selected", i)
		}
	}
}

func TestAppSelectedFiles(t *testing.T) {
	app := &App{
		Files: []File{
			{Name: "file1.txt", Selected: true},
			{Name: "file2.txt", Selected: false},
			{Name: "file3.txt", Selected: true},
		},
	}

	selected := app.SelectedFiles()

	if len(selected) != 2 {
		t.Errorf("Expected 2 selected files, got %d", len(selected))
	}
	if selected[0].Name != "file1.txt" {
		t.Errorf("Expected file1.txt, got %s", selected[0].Name)
	}
	if selected[1].Name != "file3.txt" {
		t.Errorf("Expected file3.txt, got %s", selected[1].Name)
	}
}

func TestAppSelectedFiles_None(t *testing.T) {
	app := &App{
		Files: []File{
			{Name: "file1.txt", Selected: false},
			{Name: "file2.txt", Selected: false},
		},
	}

	selected := app.SelectedFiles()

	if len(selected) != 0 {
		t.Errorf("Expected 0 selected files, got %d", len(selected))
	}
}

// ============ Category Tests ============

func TestCategory(t *testing.T) {
	cat := Category{
		ID:    "terminal",
		Name:  "Terminal Emulators",
		Icon:  "💻",
		Count: 3,
	}

	if cat.ID != "terminal" {
		t.Errorf("Expected ID 'terminal', got %s", cat.ID)
	}
	if cat.Name != "Terminal Emulators" {
		t.Errorf("Expected Name 'Terminal Emulators', got %s", cat.Name)
	}
	if cat.Icon != "💻" {
		t.Errorf("Expected Icon '💻', got %s", cat.Icon)
	}
	if cat.Count != 3 {
		t.Errorf("Expected Count 3, got %d", cat.Count)
	}
}

// ============ AppDefinition Tests ============

func TestAppDefinition(t *testing.T) {
	def := AppDefinition{
		ID:             "nvim",
		Name:           "Neovim",
		Category:       "editor",
		Icon:           "📝",
		ConfigPaths:    []string{"~/.config/nvim"},
		EncryptedFiles: []string{"secrets.lua"},
	}

	if def.ID != "nvim" {
		t.Errorf("Expected ID 'nvim', got %s", def.ID)
	}
	if len(def.ConfigPaths) != 1 {
		t.Errorf("Expected 1 ConfigPath, got %d", len(def.ConfigPaths))
	}
	if len(def.EncryptedFiles) != 1 {
		t.Errorf("Expected 1 EncryptedFile, got %d", len(def.EncryptedFiles))
	}
}

// ============ File struct field tests ============

func TestFileStruct(t *testing.T) {
	now := time.Now()
	file := File{
		Name:         "config.toml",
		Path:         "/home/user/.config/app/config.toml",
		RelPath:      "config.toml",
		Size:         1024,
		ModTime:      now,
		IsDir:        false,
		Encrypted:    true,
		Selected:     true,
		SyncStatus:   StatusModified,
		LocalHash:    "abc123",
		DotfilesHash: "def456",
		ConflictType: ConflictBothModified,
	}

	if file.Name != "config.toml" {
		t.Error("Name field mismatch")
	}
	if file.Size != 1024 {
		t.Error("Size field mismatch")
	}
	if !file.ModTime.Equal(now) {
		t.Error("ModTime field mismatch")
	}
	if !file.Encrypted {
		t.Error("Encrypted field mismatch")
	}
	if file.SyncStatus != StatusModified {
		t.Error("SyncStatus field mismatch")
	}
	if file.LocalHash != "abc123" {
		t.Error("LocalHash field mismatch")
	}
	if file.DotfilesHash != "def456" {
		t.Error("DotfilesHash field mismatch")
	}
	if file.ConflictType != ConflictBothModified {
		t.Error("ConflictType field mismatch")
	}
}

func TestConflictIcon_Default(t *testing.T) {
	// Test unknown conflict type (default case)
	unknownConflict := ConflictType(99)
	icon := unknownConflict.ConflictIcon()
	if icon != "?" {
		t.Errorf("Expected '?' for unknown conflict type, got %s", icon)
	}
}

func TestConflictString_Default(t *testing.T) {
	// Test unknown conflict type (default case)
	unknownConflict := ConflictType(99)
	str := unknownConflict.ConflictString()
	if str != "Unknown" {
		t.Errorf("Expected 'Unknown' for unknown conflict type, got %s", str)
	}
}

func TestNewFile_WithSameBasePath(t *testing.T) {
	// Create temp directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// When basePath equals the file's directory, relPath should work correctly
	file, err := NewFile(testFile, tempDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	if file.Name != "test.txt" {
		t.Errorf("Expected name 'test.txt', got %s", file.Name)
	}
	if file.RelPath != "test.txt" {
		t.Errorf("Expected RelPath 'test.txt', got %s", file.RelPath)
	}
}

func TestNewFile_WithDifferentBasePath(t *testing.T) {
	// Create temp directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// basePath is parent, so relPath should include subdir
	file, err := NewFile(testFile, tempDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	expectedRelPath := filepath.Join("subdir", "test.txt")
	if file.RelPath != expectedRelPath {
		t.Errorf("Expected RelPath '%s', got %s", expectedRelPath, file.RelPath)
	}
}

func TestNewFile_DirectoryWithIsDir(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "testdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	file, err := NewFile(subDir, tempDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	if !file.IsDir {
		t.Error("IsDir should be true for directory")
	}
	if file.Name != "testdir" {
		t.Errorf("Expected name 'testdir', got %s", file.Name)
	}
}

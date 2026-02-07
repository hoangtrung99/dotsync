package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"dotsync/internal/models"
)

func TestNew(t *testing.T) {
	s := New("")
	if s == nil {
		t.Fatal("New should return a Scanner")
	}
	if s.homeDir == "" {
		t.Error("homeDir should be set")
	}
	if s.brewApps == nil {
		t.Error("brewApps should be initialized")
	}
}

func TestNewWithConfigPath(t *testing.T) {
	configPath := "/tmp/test-config"
	s := New(configPath)
	if s.configPath != configPath {
		t.Errorf("Expected configPath %s, got %s", configPath, s.configPath)
	}
}

func TestIsBrewInstalled(t *testing.T) {
	s := New("")
	s.brewWg.Wait() // Wait for background brew loading to finish

	// Empty apps map should return false
	s.brewApps = make(map[string]bool)
	if s.IsBrewInstalled("nonexistent") {
		t.Error("IsBrewInstalled should return false for non-installed app")
	}

	// Add an app and test
	s.brewApps["testapp"] = true
	if !s.IsBrewInstalled("testapp") {
		t.Error("IsBrewInstalled should return true for installed app")
	}

	// Test case-insensitivity
	if !s.IsBrewInstalled("TestApp") {
		t.Error("IsBrewInstalled should be case-insensitive")
	}
}

func TestExpandPath(t *testing.T) {
	s := New("")

	tests := []struct {
		input    string
		contains string
	}{
		{"~/.config/test", ".config/test"},
		{"$HOME/.config/test", ".config/test"},
		{"/absolute/path", "/absolute/path"},
	}

	for _, tt := range tests {
		result := s.expandPath(tt.input)
		if result == "" {
			t.Errorf("expandPath(%s) returned empty string", tt.input)
		}
		// Check it doesn't contain ~ after expansion
		if tt.input[0] == '~' && result[0] == '~' {
			t.Errorf("expandPath(%s) didn't expand tilde, got %s", tt.input, result)
		}
	}
}

func TestPathExists(t *testing.T) {
	s := New("")

	// Test with existing path
	tempDir := t.TempDir()
	if !s.pathExists(tempDir) {
		t.Errorf("pathExists should return true for existing directory: %s", tempDir)
	}

	// Test with non-existing path
	if s.pathExists("/this/path/definitely/does/not/exist") {
		t.Error("pathExists should return false for non-existing path")
	}

	// Test with file
	tempFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(tempFile, []byte("test"), 0644)
	if !s.pathExists(tempFile) {
		t.Errorf("pathExists should return true for existing file: %s", tempFile)
	}
}

func TestShouldSkipDir(t *testing.T) {
	s := New("")

	tests := []struct {
		name string
		skip bool
	}{
		{".git", true},
		{".cache", true},
		{"cache", true},
		{"caches", true},
		{"logs", true},
		{"myapp", false},
		{"neovim", false},
	}

	for _, tt := range tests {
		result := s.shouldSkipDir(tt.name)
		if result != tt.skip {
			t.Errorf("shouldSkipDir(%s) = %v, want %v", tt.name, result, tt.skip)
		}
	}
}

func TestShouldSkip(t *testing.T) {
	s := New("")

	tests := []struct {
		name string
		skip bool
	}{
		{".DS_Store", true},
		{".git", true},
		{"node_modules", true},
		{"__pycache__", true},
		{"config.toml", false},
		{"init.lua", false},
	}

	for _, tt := range tests {
		result := s.shouldSkip(tt.name)
		if result != tt.skip {
			t.Errorf("shouldSkip(%s) = %v, want %v", tt.name, result, tt.skip)
		}
	}
}

func TestGetBuiltinDefinitions(t *testing.T) {
	s := New("")
	defs := s.getBuiltinDefinitions()

	if len(defs) == 0 {
		t.Fatal("getBuiltinDefinitions should return definitions")
	}

	// Check for some known apps
	foundApps := make(map[string]bool)
	for _, def := range defs {
		foundApps[def.ID] = true
	}

	expectedApps := []string{"nvim", "zsh", "git", "starship", "kitty"}
	for _, appID := range expectedApps {
		if !foundApps[appID] {
			t.Errorf("Expected app %s not found in definitions", appID)
		}
	}
}

func TestCollectFiles(t *testing.T) {
	s := New("")

	// Create temp directory with files
	tempDir := t.TempDir()
	testFile1 := filepath.Join(tempDir, "config.toml")
	testFile2 := filepath.Join(tempDir, "init.lua")
	os.WriteFile(testFile1, []byte("test config"), 0644)
	os.WriteFile(testFile2, []byte("test init"), 0644)

	// Create a subdirectory with file
	subDir := filepath.Join(tempDir, "subdir")
	os.MkdirAll(subDir, 0755)
	subFile := filepath.Join(subDir, "nested.txt")
	os.WriteFile(subFile, []byte("nested content"), 0644)

	files, err := s.collectFiles(tempDir, nil)
	if err != nil {
		t.Fatalf("collectFiles failed: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("Expected at least 2 files, got %d", len(files))
	}
}

func TestCollectFiles_SkipsHiddenAndCache(t *testing.T) {
	s := New("")

	tempDir := t.TempDir()

	// Create regular file
	goodFile := filepath.Join(tempDir, "config.toml")
	os.WriteFile(goodFile, []byte("good"), 0644)

	// Create files that should be skipped
	dsStore := filepath.Join(tempDir, ".DS_Store")
	os.WriteFile(dsStore, []byte("skip"), 0644)

	files, _ := s.collectFiles(tempDir, nil)

	for _, f := range files {
		if f.Name == ".DS_Store" {
			t.Error("collectFiles should skip .DS_Store")
		}
	}
}

func TestScan(t *testing.T) {
	s := New("")

	apps, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should return at least some apps on a dev machine
	// But this test might fail on a bare system
	t.Logf("Found %d apps", len(apps))
}

func TestSkipPatterns(t *testing.T) {
	expected := []string{".DS_Store", ".git", "node_modules", "__pycache__"}
	for _, pattern := range expected {
		found := false
		for _, p := range skipPatterns {
			if p == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected pattern %s not in skipPatterns", pattern)
		}
	}
}

func TestSkipDirs(t *testing.T) {
	expected := []string{"configstore", "cache", "logs", "tmp"}
	for _, dir := range expected {
		if !skipDirs[dir] {
			t.Errorf("Expected dir %s not in skipDirs", dir)
		}
	}
}

func TestGroupByCategory(t *testing.T) {
	apps := []*models.App{
		{ID: "nvim", Category: "editor"},
		{ID: "vscode", Category: "editor"},
		{ID: "zsh", Category: "shell"},
		{ID: "kitty", Category: "terminal"},
		{ID: "unknown", Category: ""},
	}

	groups := GroupByCategory(apps)

	// Check editor group
	if len(groups["editor"]) != 2 {
		t.Errorf("Expected 2 editors, got %d", len(groups["editor"]))
	}

	// Check shell group
	if len(groups["shell"]) != 1 {
		t.Errorf("Expected 1 shell, got %d", len(groups["shell"]))
	}

	// Check terminal group
	if len(groups["terminal"]) != 1 {
		t.Errorf("Expected 1 terminal, got %d", len(groups["terminal"]))
	}

	// Check empty category defaults to "other"
	if len(groups["other"]) != 1 {
		t.Errorf("Expected 1 other, got %d", len(groups["other"]))
	}
}

func TestGroupByCategory_Empty(t *testing.T) {
	groups := GroupByCategory([]*models.App{})
	if len(groups) != 0 {
		t.Errorf("Expected empty groups, got %d", len(groups))
	}
}

func TestCategoryOrder(t *testing.T) {
	order := CategoryOrder()

	if len(order) == 0 {
		t.Error("CategoryOrder should not be empty")
	}

	// Check expected categories are present
	expectedCategories := []string{"ai", "terminal", "shell", "editor", "git", "dev"}
	for _, cat := range expectedCategories {
		found := false
		for _, o := range order {
			if o == cat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected category %s in order", cat)
		}
	}
}

func TestCategoryNames(t *testing.T) {
	names := CategoryNames()

	if len(names) == 0 {
		t.Error("CategoryNames should not be empty")
	}

	// Check expected names
	expectedNames := map[string]string{
		"ai":       "AI Tools",
		"terminal": "Terminals",
		"shell":    "Shells",
		"editor":   "Editors",
		"git":      "Git",
		"dev":      "Dev Tools",
	}

	for cat, name := range expectedNames {
		if names[cat] != name {
			t.Errorf("Expected %s for %s, got %s", name, cat, names[cat])
		}
	}
}

func TestCategoryIcons(t *testing.T) {
	icons := CategoryIcons()

	if len(icons) == 0 {
		t.Error("CategoryIcons should not be empty")
	}

	// Check expected icons exist
	expectedCategories := []string{"ai", "terminal", "shell", "editor", "git", "dev"}
	for _, cat := range expectedCategories {
		if icons[cat] == "" {
			t.Errorf("Expected icon for category %s", cat)
		}
	}
}

func TestScanAll(t *testing.T) {
	s := New("")

	apps, err := s.ScanAll()
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}

	// ScanAll should return all defined apps (installed or not)
	if len(apps) == 0 {
		t.Error("ScanAll should return at least some apps")
	}

	// Should have more than just installed apps
	installedCount := 0
	for _, app := range apps {
		if app.Installed {
			installedCount++
		}
	}

	t.Logf("ScanAll: %d total apps, %d installed", len(apps), installedCount)
}

func TestLoadDefinitions_NonExistent(t *testing.T) {
	s := New("/nonexistent/path/apps.yaml")

	// loadDefinitions should return error for non-existent file
	_, err := s.loadDefinitions()
	if err == nil {
		t.Error("loadDefinitions should return error for non-existent file")
	}
}

func TestLoadDefinitions_InvalidYAML(t *testing.T) {
	// Create temp file with invalid YAML
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "apps.yaml")
	os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644)

	s := New(configPath)
	_, err := s.loadDefinitions()
	if err == nil {
		t.Error("loadDefinitions should return error for invalid YAML")
	}
}

func TestGetBuiltinDefinitions_Count(t *testing.T) {
	s := New("")
	defs := s.getBuiltinDefinitions()

	if len(defs) == 0 {
		t.Error("getBuiltinDefinitions should return definitions")
	}

	// Check we have expected number of definitions
	if len(defs) < 100 {
		t.Errorf("Expected at least 100 definitions, got %d", len(defs))
	}

	// Check some expected apps exist
	expectedApps := []string{"nvim", "zsh", "git", "starship", "kitty"}
	for _, appID := range expectedApps {
		found := false
		for _, def := range defs {
			if def.ID == appID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected app %s in definitions", appID)
		}
	}
}

func TestCollectFiles_WithSubdirectories(t *testing.T) {
	tempDir := t.TempDir()
	s := New("")

	// Create nested structure
	subDir := filepath.Join(tempDir, "subdir")
	os.MkdirAll(subDir, 0755)

	// Create files in root and subdir
	os.WriteFile(filepath.Join(tempDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644)

	files, err := s.collectFiles(tempDir, nil)
	if err != nil {
		t.Fatalf("collectFiles failed: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("Expected at least 2 files, got %d", len(files))
	}
}

func TestCollectFiles_SingleFile(t *testing.T) {
	tempDir := t.TempDir()
	s := New("")

	// Create a single file
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	files, err := s.collectFiles(testFile, nil)
	if err != nil {
		t.Fatalf("collectFiles failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}
}

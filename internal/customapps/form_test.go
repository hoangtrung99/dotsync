package customapps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildDefinition_FolderMode(t *testing.T) {
	home, _ := os.UserHomeDir()
	absPath := filepath.Join(home, ".hammerspoon")

	def, err := BuildDefinition(FormInput{
		Mode:  "folder",
		Name:  "Hammerspoon",
		Paths: []string{absPath},
	})
	if err != nil {
		t.Fatalf("BuildDefinition() error = %v", err)
	}

	if def.ID != "hammerspoon" {
		t.Fatalf("unexpected ID: %s", def.ID)
	}
	if len(def.ConfigPaths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(def.ConfigPaths))
	}
	if def.ConfigPaths[0] != "~/.hammerspoon" {
		t.Fatalf("expected normalized home path, got %s", def.ConfigPaths[0])
	}
}

func TestBuildDefinition_AppModeMultiplePaths(t *testing.T) {
	def, err := BuildDefinition(FormInput{
		Mode: "app",
		Name: "Claude Config",
		Paths: []string{
			"~/.claude/settings.json",
			"~/.claude/agents",
		},
	})
	if err != nil {
		t.Fatalf("BuildDefinition() error = %v", err)
	}

	if len(def.ConfigPaths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(def.ConfigPaths))
	}
	if def.Category != "custom" {
		t.Fatalf("expected custom category, got %s", def.Category)
	}
}

func TestBuildDefinition_RejectsEmptyName(t *testing.T) {
	_, err := BuildDefinition(FormInput{Mode: "folder", Name: "", Paths: []string{"~/.x"}})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestBuildDefinition_RejectsMissingPath(t *testing.T) {
	_, err := BuildDefinition(FormInput{Mode: "folder", Name: "X", Paths: nil})
	if err == nil {
		t.Fatal("expected error for missing paths")
	}
}

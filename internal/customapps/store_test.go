package customapps

import (
	"os"
	"path/filepath"
	"testing"

	"dotsync/internal/models"

	"gopkg.in/yaml.v3"
)

func TestStore_LoadMissingFile_ReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	store := New(filepath.Join(tmp, "missing.yaml"))

	defs, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected empty definitions, got %d", len(defs))
	}
}

func TestStore_AddFolderEntry_PersistsAsAppDefinition(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "apps.yaml")
	store := New(cfgPath)

	def := models.AppDefinition{
		ID:          "custom-hammerspoon",
		Name:        "Hammerspoon Custom",
		Category:    "custom",
		Icon:        "📁",
		ConfigPaths: []string{"~/.hammerspoon"},
	}
	if err := store.Add(def); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read file error = %v", err)
	}

	var cfg models.AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml unmarshal error = %v", err)
	}

	if len(cfg.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
	}
	if cfg.Apps[0].ID != "custom-hammerspoon" {
		t.Fatalf("unexpected ID: %s", cfg.Apps[0].ID)
	}
}

func TestStore_AddAppEntry_WithMultiplePaths(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "apps.yaml")
	store := New(cfgPath)

	def := models.AppDefinition{
		ID:       "custom-claude-plus",
		Name:     "Claude Plus",
		Category: "custom",
		Icon:     "📁",
		ConfigPaths: []string{
			"~/.claude/settings.json",
			"~/.claude/agents",
		},
	}
	if err := store.Add(def); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	defs, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if len(defs[0].ConfigPaths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(defs[0].ConfigPaths))
	}
}

func TestStore_AddDuplicateID_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "apps.yaml")
	store := New(cfgPath)

	def := models.AppDefinition{
		ID:          "custom-dup",
		Name:        "Dup",
		Category:    "custom",
		Icon:        "📁",
		ConfigPaths: []string{"~/.dup"},
	}
	if err := store.Add(def); err != nil {
		t.Fatalf("first Add() should succeed: %v", err)
	}
	if err := store.Add(def); err == nil {
		t.Fatalf("expected duplicate error, got nil")
	}
}

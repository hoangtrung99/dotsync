package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	// Test all key bindings are defined
	bindings := []struct {
		name    string
		binding key.Binding
	}{
		{"Up", km.Up},
		{"Down", km.Down},
		{"Left", km.Left},
		{"Right", km.Right},
		{"Tab", km.Tab},
		{"ShiftTab", km.ShiftTab},
		{"Space", km.Space},
		{"Enter", km.Enter},
		{"SelectAll", km.SelectAll},
		{"DeselectAll", km.DeselectAll},
		{"Push", km.Push},
		{"Pull", km.Pull},
		{"Scan", km.Scan},
		{"Brewfile", km.Brewfile},
		{"Help", km.Help},
		{"Quit", km.Quit},
		{"Escape", km.Escape},
		{"Diff", km.Diff},
		{"Git", km.Git},
		{"Merge", km.Merge},
		{"NextHunk", km.NextHunk},
		{"PrevHunk", km.PrevHunk},
		{"KeepLocal", km.KeepLocal},
		{"UseDotfiles", km.UseDotfiles},
		{"AddCustom", km.AddCustom},
	}

	for _, b := range bindings {
		if len(b.binding.Keys()) == 0 {
			t.Errorf("%s binding should have keys", b.name)
		}
		if b.binding.Help().Key == "" {
			t.Errorf("%s binding should have help key", b.name)
		}
		if b.binding.Help().Desc == "" {
			t.Errorf("%s binding should have help description", b.name)
		}
	}
}

func TestDefaultKeyMap_HasAddCustomBinding(t *testing.T) {
	km := DefaultKeyMap()
	if len(km.AddCustom.Keys()) == 0 {
		t.Fatalf("AddCustom should define at least one key")
	}
	if km.AddCustom.Keys()[0] != "+" {
		t.Fatalf("AddCustom key should be '+', got %q", km.AddCustom.Keys()[0])
	}
	if km.AddCustom.Help().Desc == "" {
		t.Fatal("AddCustom should have help description")
	}
}

func TestFullHelp_IncludesAddCustomBinding(t *testing.T) {
	km := DefaultKeyMap()
	groups := km.FullHelp()

	found := false
	for _, group := range groups {
		for _, b := range group {
			if len(b.Keys()) > 0 && b.Keys()[0] == "+" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		t.Fatal("FullHelp should include AddCustom binding")
	}
}

func TestKeyMap_Up(t *testing.T) {
	km := DefaultKeyMap()
	keys := km.Up.Keys()

	if len(keys) != 2 {
		t.Errorf("Up should have 2 keys, got %d", len(keys))
	}

	// Check for "up" and "k"
	hasUp := false
	hasK := false
	for _, k := range keys {
		if k == "up" {
			hasUp = true
		}
		if k == "k" {
			hasK = true
		}
	}

	if !hasUp {
		t.Error("Up should include 'up' key")
	}
	if !hasK {
		t.Error("Up should include 'k' key")
	}
}

func TestKeyMap_Down(t *testing.T) {
	km := DefaultKeyMap()
	keys := km.Down.Keys()

	if len(keys) != 2 {
		t.Errorf("Down should have 2 keys, got %d", len(keys))
	}
}

func TestKeyMap_Quit(t *testing.T) {
	km := DefaultKeyMap()
	keys := km.Quit.Keys()

	// Should have "q" and "ctrl+c"
	if len(keys) != 2 {
		t.Errorf("Quit should have 2 keys, got %d", len(keys))
	}
}

func TestKeyMap_ShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	help := km.ShortHelp()

	if len(help) == 0 {
		t.Error("ShortHelp should not be empty")
	}

	// Should include common actions
	expectedCount := 7 // Space, Tab, Push, Pull, Scan, Quit, and one more for new features
	if len(help) != expectedCount {
		t.Errorf("ShortHelp should have %d bindings, got %d", expectedCount, len(help))
	}
}

func TestKeyMap_FullHelp(t *testing.T) {
	km := DefaultKeyMap()
	help := km.FullHelp()

	if len(help) == 0 {
		t.Error("FullHelp should not be empty")
	}

	// Should have multiple groups
	if len(help) < 4 {
		t.Errorf("FullHelp should have at least 4 groups, got %d", len(help))
	}

	// Each group should have bindings
	for i, group := range help {
		if len(group) == 0 {
			t.Errorf("FullHelp group %d should not be empty", i)
		}
	}
}

func TestKeyMap_Navigation(t *testing.T) {
	km := DefaultKeyMap()

	// Test navigation keys have vim-style alternatives
	navKeys := []struct {
		name    string
		binding key.Binding
		vimKey  string
	}{
		{"Up", km.Up, "k"},
		{"Down", km.Down, "j"},
		{"Left", km.Left, "h"},
		{"Right", km.Right, "l"},
	}

	for _, nk := range navKeys {
		keys := nk.binding.Keys()
		hasVimKey := false
		for _, k := range keys {
			if k == nk.vimKey {
				hasVimKey = true
				break
			}
		}
		if !hasVimKey {
			t.Errorf("%s should include vim key '%s'", nk.name, nk.vimKey)
		}
	}
}

func TestKeyMap_DiffMergeKeys(t *testing.T) {
	km := DefaultKeyMap()

	// Diff key should be 'd'
	if km.Diff.Keys()[0] != "d" {
		t.Errorf("Diff key should be 'd', got '%s'", km.Diff.Keys()[0])
	}

	// Merge key should be 'm'
	if km.Merge.Keys()[0] != "m" {
		t.Errorf("Merge key should be 'm', got '%s'", km.Merge.Keys()[0])
	}

	// NextHunk should be 'n'
	if km.NextHunk.Keys()[0] != "n" {
		t.Errorf("NextHunk key should be 'n', got '%s'", km.NextHunk.Keys()[0])
	}

	// PrevHunk should be 'N'
	if km.PrevHunk.Keys()[0] != "N" {
		t.Errorf("PrevHunk key should be 'N', got '%s'", km.PrevHunk.Keys()[0])
	}

	// KeepLocal should be '1'
	if km.KeepLocal.Keys()[0] != "1" {
		t.Errorf("KeepLocal key should be '1', got '%s'", km.KeepLocal.Keys()[0])
	}

	// UseDotfiles should be '2'
	if km.UseDotfiles.Keys()[0] != "2" {
		t.Errorf("UseDotfiles key should be '2', got '%s'", km.UseDotfiles.Keys()[0])
	}
}

func TestKeyMap_GitKey(t *testing.T) {
	km := DefaultKeyMap()

	if km.Git.Keys()[0] != "g" {
		t.Errorf("Git key should be 'g', got '%s'", km.Git.Keys()[0])
	}
}

func TestKeyMap_SyncKeys(t *testing.T) {
	km := DefaultKeyMap()

	// Push should be 'p'
	if km.Push.Keys()[0] != "p" {
		t.Errorf("Push key should be 'p', got '%s'", km.Push.Keys()[0])
	}

	// Pull should be 'l' (mnemonic: 'l' for load/pull)
	if km.Pull.Keys()[0] != "l" {
		t.Errorf("Pull key should be 'l', got '%s'", km.Pull.Keys()[0])
	}
}

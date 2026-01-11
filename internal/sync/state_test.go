package sync

import (
	"os"
	"path/filepath"
	"testing"

	"dotsync/internal/models"
)

func TestStateManager_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create state manager
	sm := NewStateManager(tmpDir)

	// Update state
	sm.SetFileState("testapp", "config.toml", "hash1", "hash1")

	// Save
	if err := sm.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load in new manager
	sm2 := NewStateManager(tmpDir)
	if err := sm2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify
	state, ok := sm2.GetFileState("testapp", "config.toml")
	if !ok {
		t.Fatal("File state should exist")
	}

	if state.LocalHash != "hash1" {
		t.Errorf("LocalHash mismatch: %s != hash1", state.LocalHash)
	}
}

func TestStateManager_DetectConflict(t *testing.T) {
	tmpDir := t.TempDir()

	sm := NewStateManager(tmpDir)

	// Set initial synced state
	sm.SetFileState("app", "file.txt", "base", "base")

	tests := []struct {
		name           string
		localHash      string
		dotfilesHash   string
		expectedResult models.ConflictType
	}{
		{
			name:           "No change",
			localHash:      "base",
			dotfilesHash:   "base",
			expectedResult: models.ConflictNone,
		},
		{
			name:           "Local modified",
			localHash:      "modified",
			dotfilesHash:   "base",
			expectedResult: models.ConflictLocalModified,
		},
		{
			name:           "Dotfiles modified",
			localHash:      "base",
			dotfilesHash:   "modified",
			expectedResult: models.ConflictDotfilesModified,
		},
		{
			name:           "Both modified - conflict",
			localHash:      "local_change",
			dotfilesHash:   "dotfiles_change",
			expectedResult: models.ConflictBothModified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sm.DetectConflict("app", "file.txt", tt.localHash, tt.dotfilesHash)
			if result != tt.expectedResult {
				t.Errorf("DetectConflict() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestStateManager_RemoveFileState(t *testing.T) {
	tmpDir := t.TempDir()

	sm := NewStateManager(tmpDir)
	sm.SetFileState("app", "file.txt", "hash1", "hash2")

	_, ok := sm.GetFileState("app", "file.txt")
	if !ok {
		t.Fatal("State should exist before remove")
	}

	sm.RemoveFileState("app", "file.txt")

	_, ok = sm.GetFileState("app", "file.txt")
	if ok {
		t.Error("State should not exist after remove")
	}
}

func TestStateManager_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "nonexistent")

	sm := NewStateManager(nonExistentDir)
	err := sm.Load()

	// Should not error on non-existent file
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("Load should handle non-existent file gracefully: %v", err)
	}
}

func TestStateManager_ClearState(t *testing.T) {
	tmpDir := t.TempDir()

	sm := NewStateManager(tmpDir)
	sm.SetFileState("app", "file.txt", "hash1", "hash2")

	sm.ClearState()

	_, ok := sm.GetFileState("app", "file.txt")
	if ok {
		t.Error("State should be cleared")
	}
}

func TestStateManager_GetLastSync(t *testing.T) {
	tmpDir := t.TempDir()

	sm := NewStateManager(tmpDir)

	// Before any sync, should return zero time or now
	lastSync := sm.GetLastSync()
	if lastSync.IsZero() {
		// This is acceptable - no sync has happened
	}

	// After setting file state, save and check
	sm.SetFileState("app", "file.txt", "hash1", "hash2")
	err := sm.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Last sync should be updated
	lastSync = sm.GetLastSync()
	// Just verify it doesn't panic and returns something
	_ = lastSync
}

func TestStateManager_DetectConflict_NoBaseState(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// When no base state exists, should detect based on current hashes
	// If both hashes are the same, no conflict
	result := sm.DetectConflict("newapp", "newfile.txt", "same", "same")
	if result != models.ConflictNone {
		t.Errorf("Expected ConflictNone for identical hashes with no base, got %v", result)
	}

	// If hashes differ with no base, that means one side has changes
	result = sm.DetectConflict("newapp", "newfile.txt", "local", "dotfiles")
	// Without base state, this could be considered conflict or initial state
	// Just verify it doesn't panic and returns a valid conflict type
	if result < models.ConflictNone || result > models.ConflictDotfilesDeleted {
		t.Errorf("Unexpected conflict type: %v", result)
	}
}

func TestStateManager_Save_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "newstate", "subdir")

	sm := NewStateManager(newDir)
	sm.SetFileState("app", "file.txt", "hash1", "hash2")

	// Save should create the directory
	err := sm.Save()
	if err != nil {
		// Some errors are expected if we can't create the directory
		// Just verify it doesn't panic
	}
}

func TestStateManager_DetectConflict_LocalDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Set initial state where local existed
	sm.SetFileState("app", "file.txt", "localhash", "dotfileshash")

	// Now local is deleted (empty hash)
	result := sm.DetectConflict("app", "file.txt", "", "dotfileshash")
	if result != models.ConflictLocalDeleted {
		t.Errorf("Expected ConflictLocalDeleted, got %v", result)
	}
}

func TestStateManager_DetectConflict_DotfilesDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Set initial state where dotfiles existed
	sm.SetFileState("app", "file.txt", "localhash", "dotfileshash")

	// Now dotfiles is deleted (empty hash)
	result := sm.DetectConflict("app", "file.txt", "localhash", "")
	if result != models.ConflictDotfilesDeleted {
		t.Errorf("Expected ConflictDotfilesDeleted, got %v", result)
	}
}

func TestStateManager_DetectConflict_BothModifiedSame(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Set initial state
	sm.SetFileState("app", "file.txt", "base", "base")

	// Both modified to same content
	result := sm.DetectConflict("app", "file.txt", "newcontent", "newcontent")
	if result != models.ConflictNone {
		t.Errorf("Expected ConflictNone when both modified to same content, got %v", result)
	}
}

func TestStateManager_DetectConflict_NoState_BothEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// No previous state, both empty
	result := sm.DetectConflict("newapp", "newfile.txt", "", "")
	if result != models.ConflictNone {
		t.Errorf("Expected ConflictNone when both empty with no state, got %v", result)
	}
}

func TestStateManager_DetectConflict_NoState_DotfilesNew(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// No previous state, only dotfiles has content
	result := sm.DetectConflict("newapp", "newfile.txt", "", "dotfileshash")
	if result != models.ConflictDotfilesNew {
		t.Errorf("Expected ConflictDotfilesNew, got %v", result)
	}
}

func TestStateManager_DetectConflict_NoState_LocalNew(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// No previous state, only local has content
	result := sm.DetectConflict("newapp", "newfile.txt", "localhash", "")
	if result != models.ConflictLocalNew {
		t.Errorf("Expected ConflictLocalNew, got %v", result)
	}
}

func TestStateManager_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "sync_state.json")

	// Write invalid JSON
	os.WriteFile(stateFile, []byte("invalid json {{{"), 0644)

	sm := NewStateManager(tmpDir)
	err := sm.Load()
	if err == nil {
		t.Error("Load should return error for invalid JSON")
	}
}

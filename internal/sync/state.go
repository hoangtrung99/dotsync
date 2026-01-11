package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"dotsync/internal/models"
)

// SyncState tracks the state of synced files for conflict detection
type SyncState struct {
	LastSync time.Time            `json:"last_sync"`
	Files    map[string]FileState `json:"files"`
}

// FileState tracks the state of a single file
type FileState struct {
	AppID        string    `json:"app_id"`
	RelPath      string    `json:"rel_path"`
	LocalHash    string    `json:"local_hash"`
	DotfilesHash string    `json:"dotfiles_hash"`
	SyncedAt     time.Time `json:"synced_at"`
}

// StateManager handles loading and saving sync state
type StateManager struct {
	statePath string
	state     *SyncState
}

// NewStateManager creates a new StateManager
func NewStateManager(configDir string) *StateManager {
	statePath := filepath.Join(configDir, "sync_state.json")
	return &StateManager{
		statePath: statePath,
		state: &SyncState{
			Files: make(map[string]FileState),
		},
	}
}

// Load loads the sync state from disk
func (s *StateManager) Load() error {
	data, err := os.ReadFile(s.statePath)
	if os.IsNotExist(err) {
		// No state file yet - that's OK
		return nil
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, s.state)
}

// Save saves the sync state to disk
func (s *StateManager) Save() error {
	// Ensure directory exists
	dir := filepath.Dir(s.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.statePath, data, 0644)
}

// GetFileState returns the state for a specific file
func (s *StateManager) GetFileState(appID, relPath string) (FileState, bool) {
	key := appID + "/" + relPath
	state, ok := s.state.Files[key]
	return state, ok
}

// SetFileState updates the state for a specific file
func (s *StateManager) SetFileState(appID, relPath, localHash, dotfilesHash string) {
	key := appID + "/" + relPath
	s.state.Files[key] = FileState{
		AppID:        appID,
		RelPath:      relPath,
		LocalHash:    localHash,
		DotfilesHash: dotfilesHash,
		SyncedAt:     time.Now(),
	}
	s.state.LastSync = time.Now()
}

// RemoveFileState removes the state for a file
func (s *StateManager) RemoveFileState(appID, relPath string) {
	key := appID + "/" + relPath
	delete(s.state.Files, key)
}

// DetectConflict determines the conflict type for a file
func (s *StateManager) DetectConflict(appID, relPath, currentLocalHash, currentDotfilesHash string) models.ConflictType {
	savedState, exists := s.GetFileState(appID, relPath)

	// No previous state
	if !exists {
		if currentLocalHash == "" && currentDotfilesHash == "" {
			return models.ConflictNone
		}
		if currentLocalHash == "" {
			return models.ConflictDotfilesNew
		}
		if currentDotfilesHash == "" {
			return models.ConflictLocalNew
		}
		if currentLocalHash == currentDotfilesHash {
			return models.ConflictNone
		}
		// First time seeing both - treat as conflict
		return models.ConflictBothModified
	}

	// Check what changed since last sync
	localChanged := currentLocalHash != savedState.LocalHash
	dotfilesChanged := currentDotfilesHash != savedState.DotfilesHash

	// Handle deletions
	if currentLocalHash == "" && savedState.LocalHash != "" {
		return models.ConflictLocalDeleted
	}
	if currentDotfilesHash == "" && savedState.DotfilesHash != "" {
		return models.ConflictDotfilesDeleted
	}

	// Check for modifications
	if localChanged && dotfilesChanged {
		// Both changed - but are they the same?
		if currentLocalHash == currentDotfilesHash {
			return models.ConflictNone // Same content
		}
		return models.ConflictBothModified
	}

	if localChanged {
		return models.ConflictLocalModified
	}

	if dotfilesChanged {
		return models.ConflictDotfilesModified
	}

	return models.ConflictNone
}

// GetLastSync returns the time of last sync
func (s *StateManager) GetLastSync() time.Time {
	return s.state.LastSync
}

// ClearState clears all state (for testing or reset)
func (s *StateManager) ClearState() {
	s.state = &SyncState{
		Files: make(map[string]FileState),
	}
}

package quicksync

import (
	"os"
	"path/filepath"

	"dotsync/internal/config"
	"dotsync/internal/modes"
	"dotsync/internal/models"
	"dotsync/internal/sync"
)

// FileState represents the current state of a file
type FileState int

const (
	// StateSynced - both local and dotfiles are the same
	StateSynced FileState = iota
	// StateLocalModified - only local changed since last sync
	StateLocalModified
	// StateRemoteModified - only dotfiles changed since last sync
	StateRemoteModified
	// StateConflict - both local and dotfiles changed
	StateConflict
	// StateLocalNew - file exists locally but not in dotfiles
	StateLocalNew
	// StateRemoteNew - file exists in dotfiles but not locally
	StateRemoteNew
	// StateDeleted - file was deleted
	StateDeleted
)

// String returns a human-readable string for the state
func (s FileState) String() string {
	switch s {
	case StateSynced:
		return "synced"
	case StateLocalModified:
		return "local modified"
	case StateRemoteModified:
		return "remote modified"
	case StateConflict:
		return "conflict"
	case StateLocalNew:
		return "local new"
	case StateRemoteNew:
		return "remote new"
	case StateDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// Icon returns an icon for the state
func (s FileState) Icon() string {
	switch s {
	case StateSynced:
		return "✓"
	case StateLocalModified:
		return "↑"
	case StateRemoteModified:
		return "↓"
	case StateConflict:
		return "⚡"
	case StateLocalNew:
		return "+"
	case StateRemoteNew:
		return "↓"
	case StateDeleted:
		return "✗"
	default:
		return "?"
	}
}

// FileInfo contains file path and state information
type FileInfo struct {
	AppID        string
	FilePath     string    // Local file path
	DotfilesPath string    // Path in dotfiles repo (backup path)
	SyncPath     string    // Path for shared copy (only when synced)
	State        FileState // Current state
	Synced       bool      // Whether sync is enabled for this file
	LocalHash    string    // Hash of local file
	RemoteHash   string    // Hash of dotfiles file
}

// DetectionResult contains the result of state detection
type DetectionResult struct {
	// Summary counts
	SyncedCount   int
	LocalModified int
	RemoteUpdated int
	Conflicts     int

	// All files grouped by state
	Synced         []FileInfo
	LocalModFiles  []FileInfo
	RemoteModFiles []FileInfo
	ConflictFiles  []FileInfo

	// Grouped by mode
	BackupFiles []FileInfo // Files in backup mode
	SyncFiles   []FileInfo // Files in sync mode
}

// HasChanges returns true if there are any changes
func (r *DetectionResult) HasChanges() bool {
	return r.LocalModified > 0 || r.RemoteUpdated > 0 || r.Conflicts > 0
}

// IsAllSynced returns true if everything is in sync
func (r *DetectionResult) IsAllSynced() bool {
	return !r.HasChanges()
}

// ConflictDetector detects sync state for files
type ConflictDetector struct {
	config       *config.Config
	modesConfig  *modes.ModesConfig
	stateManager *sync.StateManager
}

// NewConflictDetector creates a new ConflictDetector
func NewConflictDetector(cfg *config.Config, modesCfg *modes.ModesConfig) *ConflictDetector {
	stateManager := sync.NewStateManager(config.ConfigDir())
	stateManager.Load()

	return &ConflictDetector{
		config:       cfg,
		modesConfig:  modesCfg,
		stateManager: stateManager,
	}
}

// DetectAll detects state for all files in the given apps
func (d *ConflictDetector) DetectAll(apps []*models.App) *DetectionResult {
	result := &DetectionResult{
		Synced:         []FileInfo{},
		LocalModFiles:  []FileInfo{},
		RemoteModFiles: []FileInfo{},
		ConflictFiles:  []FileInfo{},
		BackupFiles:    []FileInfo{},
		SyncFiles:      []FileInfo{},
	}

	for _, app := range apps {
		if !app.Selected {
			continue
		}

		for _, file := range app.Files {
			if !file.Selected {
				continue
			}

			fileInfo := d.detectFileState(app.ID, file)

			// Group by state
			switch fileInfo.State {
			case StateSynced:
				result.Synced = append(result.Synced, fileInfo)
				result.SyncedCount++
			case StateLocalModified, StateLocalNew:
				result.LocalModFiles = append(result.LocalModFiles, fileInfo)
				result.LocalModified++
			case StateRemoteModified, StateRemoteNew:
				result.RemoteModFiles = append(result.RemoteModFiles, fileInfo)
				result.RemoteUpdated++
			case StateConflict:
				result.ConflictFiles = append(result.ConflictFiles, fileInfo)
				result.Conflicts++
			}

			// Group by mode: all files go to backup, synced files also in sync list
			result.BackupFiles = append(result.BackupFiles, fileInfo)
			if fileInfo.Synced {
				result.SyncFiles = append(result.SyncFiles, fileInfo)
			}
		}
	}

	return result
}

// DetectApp detects state for all files in a single app
func (d *ConflictDetector) DetectApp(app *models.App) *DetectionResult {
	return d.DetectAll([]*models.App{app})
}

// detectFileState detects the state of a single file
func (d *ConflictDetector) detectFileState(appID string, file models.File) FileInfo {
	synced := d.modesConfig.IsSynced(appID, file.Path)

	// Always use backup path as the primary dotfiles path
	dotfilesPath := d.modesConfig.GetBackupPath(d.config.DotfilesPath, appID, file.Path)

	// Sync path (shared copy) only when synced
	var syncPath string
	if synced {
		syncPath = d.modesConfig.GetSyncPath(d.config.DotfilesPath, appID, file.Path)
	}

	info := FileInfo{
		AppID:        appID,
		FilePath:     file.Path,
		DotfilesPath: dotfilesPath,
		SyncPath:     syncPath,
		Synced:       synced,
	}

	// Check file existence
	localExists := fileExists(file.Path)
	remoteExists := fileExists(dotfilesPath)

	// Determine state
	if !localExists && !remoteExists {
		info.State = StateDeleted
		return info
	}

	if !localExists {
		info.State = StateRemoteNew
		return info
	}

	if !remoteExists {
		info.State = StateLocalNew
		return info
	}

	// Both exist - compute hashes for comparison
	localHash, _ := sync.ComputeFileHash(file.Path)
	remoteHash, _ := sync.ComputeFileHash(dotfilesPath)

	info.LocalHash = localHash
	info.RemoteHash = remoteHash

	// Same content - synced
	if localHash == remoteHash {
		info.State = StateSynced
		return info
	}

	// Different content - use state manager to determine who changed
	conflictType := d.stateManager.DetectConflict(appID, file.RelPath, localHash, remoteHash)

	switch conflictType {
	case models.ConflictNone:
		info.State = StateSynced
	case models.ConflictLocalModified, models.ConflictLocalNew:
		info.State = StateLocalModified
	case models.ConflictDotfilesModified, models.ConflictDotfilesNew:
		info.State = StateRemoteModified
	case models.ConflictBothModified:
		info.State = StateConflict
	case models.ConflictLocalDeleted:
		info.State = StateDeleted
	case models.ConflictDotfilesDeleted:
		info.State = StateRemoteNew
	default:
		// Fallback: if hashes differ, treat as conflict
		info.State = StateConflict
	}

	return info
}

// GetStateManager returns the state manager for updating sync state
func (d *ConflictDetector) GetStateManager() *sync.StateManager {
	return d.stateManager
}

// SaveState saves the current sync state
func (d *ConflictDetector) SaveState() error {
	return d.stateManager.Save()
}

// UpdateFileState updates the sync state for a file after sync
func (d *ConflictDetector) UpdateFileState(appID, relPath, localHash, dotfilesHash string) {
	d.stateManager.SetFileState(appID, relPath, localHash, dotfilesHash)
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetBackupFilesWithChanges returns backup files that have local changes
func (r *DetectionResult) GetBackupFilesWithChanges() []FileInfo {
	var files []FileInfo
	for _, f := range r.BackupFiles {
		if f.State == StateLocalModified || f.State == StateLocalNew {
			files = append(files, f)
		}
	}
	return files
}

// GetSyncFilesWithChanges returns sync files that have any changes
func (r *DetectionResult) GetSyncFilesWithChanges() []FileInfo {
	var files []FileInfo
	for _, f := range r.SyncFiles {
		if f.State != StateSynced {
			files = append(files, f)
		}
	}
	return files
}

// CountByMode returns counts grouped by mode
func (r *DetectionResult) CountByMode() (backupChanged, syncChanged int) {
	for _, f := range r.BackupFiles {
		if f.State != StateSynced {
			backupChanged++
		}
	}
	for _, f := range r.SyncFiles {
		if f.State != StateSynced {
			syncChanged++
		}
	}
	return
}

// GetAppIDs returns unique app IDs from all changed files
func (r *DetectionResult) GetAppIDs() []string {
	seen := make(map[string]bool)
	var appIDs []string

	addFile := func(f FileInfo) {
		if !seen[f.AppID] {
			seen[f.AppID] = true
			appIDs = append(appIDs, f.AppID)
		}
	}

	for _, f := range r.LocalModFiles {
		addFile(f)
	}
	for _, f := range r.RemoteModFiles {
		addFile(f)
	}
	for _, f := range r.ConflictFiles {
		addFile(f)
	}

	return appIDs
}

// Summary returns a human-readable summary
func (r *DetectionResult) Summary() string {
	if r.IsAllSynced() {
		return "All files synced"
	}

	parts := []string{}
	if r.LocalModified > 0 {
		parts = append(parts, filepath.Join(string(rune('0'+r.LocalModified)), " local modified"))
	}
	if r.RemoteUpdated > 0 {
		parts = append(parts, filepath.Join(string(rune('0'+r.RemoteUpdated)), " remote updated"))
	}
	if r.Conflicts > 0 {
		parts = append(parts, filepath.Join(string(rune('0'+r.Conflicts)), " conflicts"))
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}

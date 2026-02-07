package modes

import (
	"path/filepath"
	"strings"
)

// GetMode returns the mode for a specific file within an app
// Priority: file override > app setting > default_mode
func (m *ModesConfig) GetMode(appID, filePath string) Mode {
	// 1. Check file-level override first (highest priority)
	// Try multiple path formats for flexibility
	fileKey := normalizeFilePath(appID, filePath)
	if mode, ok := m.Files[fileKey]; ok {
		return mode
	}

	// Also try with just the filename
	if mode, ok := m.Files[filePath]; ok {
		return mode
	}

	// 2. Check app-level setting
	if mode, ok := m.Apps[appID]; ok {
		return mode
	}

	// 3. Fall back to default mode
	return m.DefaultMode
}

// GetAppMode returns the mode for an entire app
// Priority: app setting > default_mode
func (m *ModesConfig) GetAppMode(appID string) Mode {
	if mode, ok := m.Apps[appID]; ok {
		return mode
	}
	return m.DefaultMode
}

// GetFileMode returns the mode for a specific file
// This is an alias for GetMode for backward compatibility
func (m *ModesConfig) GetFileMode(appID, filePath string) Mode {
	return m.GetMode(appID, filePath)
}

// HasAppOverride returns true if the app has a mode override
func (m *ModesConfig) HasAppOverride(appID string) bool {
	_, ok := m.Apps[appID]
	return ok
}

// HasFileOverride returns true if the file has a mode override
func (m *ModesConfig) HasFileOverride(appID, filePath string) bool {
	fileKey := normalizeFilePath(appID, filePath)
	if _, ok := m.Files[fileKey]; ok {
		return true
	}
	if _, ok := m.Files[filePath]; ok {
		return true
	}
	return false
}

// GetEffectiveMode returns the mode and its source for debugging
func (m *ModesConfig) GetEffectiveMode(appID, filePath string) (Mode, string) {
	// 1. Check file-level override
	fileKey := normalizeFilePath(appID, filePath)
	if mode, ok := m.Files[fileKey]; ok {
		return mode, "file"
	}
	if mode, ok := m.Files[filePath]; ok {
		return mode, "file"
	}

	// 2. Check app-level setting
	if mode, ok := m.Apps[appID]; ok {
		return mode, "app"
	}

	// 3. Default mode
	return m.DefaultMode, "default"
}

// ToggleAppMode toggles the mode for an app between sync and backup
func (m *ModesConfig) ToggleAppMode(appID string) Mode {
	currentMode := m.GetAppMode(appID)
	newMode := currentMode.Toggle()
	m.SetAppMode(appID, newMode)
	return newMode
}

// ToggleFileMode toggles the mode for a file between sync and backup
func (m *ModesConfig) ToggleFileMode(appID, filePath string) Mode {
	currentMode := m.GetMode(appID, filePath)
	newMode := currentMode.Toggle()
	fileKey := normalizeFilePath(appID, filePath)
	m.SetFileMode(fileKey, newMode)
	return newMode
}

// SetAllAppsMode sets the mode for all apps in the provided list
func (m *ModesConfig) SetAllAppsMode(appIDs []string, mode Mode) {
	for _, appID := range appIDs {
		m.SetAppMode(appID, mode)
	}
}

// normalizeFilePath creates a consistent file key for the files map
// Format: appID/filename (e.g., "zsh/.zshrc")
func normalizeFilePath(appID, filePath string) string {
	// If filePath already contains the appID prefix, return as-is
	if strings.HasPrefix(filePath, appID+"/") {
		return filePath
	}

	// Get just the filename from the path
	filename := filepath.Base(filePath)

	// Combine with appID
	return appID + "/" + filename
}

// GetBackupPath returns the storage path for a backup mode file
// Format: dotfiles/{app}/{machine}/{file}
func (m *ModesConfig) GetBackupPath(basePath, appID, filePath string) string {
	filename := filepath.Base(filePath)
	return filepath.Join(basePath, appID, m.MachineName, filename)
}

// GetSyncPath returns the storage path for a sync mode file
// Format: dotfiles/{app}/{file}
func (m *ModesConfig) GetSyncPath(basePath, appID, filePath string) string {
	filename := filepath.Base(filePath)
	return filepath.Join(basePath, appID, filename)
}

// GetStoragePath returns the appropriate storage path based on mode
func (m *ModesConfig) GetStoragePath(basePath, appID, filePath string) string {
	mode := m.GetMode(appID, filePath)
	if mode == ModeBackup {
		return m.GetBackupPath(basePath, appID, filePath)
	}
	return m.GetSyncPath(basePath, appID, filePath)
}

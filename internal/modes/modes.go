package modes

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ModesConfig holds the mode configuration for apps and files.
// In v2, every file is always backed up per-machine.
// Sync is an additional opt-in: synced files also get a shared copy.
type ModesConfig struct {
	Version     int             `json:"version"`
	MachineName string          `json:"machine_name"`
	SyncedApps  map[string]bool `json:"synced_apps"`  // appID -> true = sync ON
	SyncedFiles map[string]bool `json:"synced_files"` // "appID/file" -> true
}

// configFileName is the name of the modes config file
const configFileName = "modes.json"

// ConfigPath returns the path to the modes config file
func ConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "dotsync", configFileName)
}

// Default returns the default modes configuration
func Default() *ModesConfig {
	hostname, _ := os.Hostname()

	return &ModesConfig{
		Version:     2,
		MachineName: hostname,
		SyncedApps:  make(map[string]bool),
		SyncedFiles: make(map[string]bool),
	}
}

// v1Config is the old config format for migration
type v1Config struct {
	MachineName string            `json:"machine_name"`
	DefaultMode string            `json:"default_mode"`
	Apps        map[string]string `json:"apps"`
	Files       map[string]string `json:"files"`
}

// Load loads the modes configuration from file
func Load() (*ModesConfig, error) {
	configPath := ConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	// Check if this is v1 format (no "version" field or version == 0)
	var versionCheck struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &versionCheck); err != nil {
		return nil, err
	}

	if versionCheck.Version < 2 {
		return migrateV1(data)
	}

	var cfg ModesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Ensure maps are initialized
	if cfg.SyncedApps == nil {
		cfg.SyncedApps = make(map[string]bool)
	}
	if cfg.SyncedFiles == nil {
		cfg.SyncedFiles = make(map[string]bool)
	}

	return &cfg, nil
}

// migrateV1 migrates v1 config format to v2
func migrateV1(data []byte) (*ModesConfig, error) {
	var v1 v1Config
	if err := json.Unmarshal(data, &v1); err != nil {
		return nil, err
	}

	cfg := Default()
	cfg.MachineName = v1.MachineName
	if cfg.MachineName == "" {
		hostname, _ := os.Hostname()
		cfg.MachineName = hostname
	}

	// Apps with mode "sync" → synced
	for appID, mode := range v1.Apps {
		if mode == "sync" {
			cfg.SyncedApps[appID] = true
		}
	}

	// Files with mode "sync" → synced
	for fileKey, mode := range v1.Files {
		if mode == "sync" {
			cfg.SyncedFiles[fileKey] = true
		}
	}

	// Save migrated config
	if err := cfg.Save(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Save saves the modes configuration to file
func (m *ModesConfig) Save() error {
	configPath := ConfigPath()

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// IsSynced returns true if a specific file has sync enabled
func (m *ModesConfig) IsSynced(appID, filePath string) bool {
	// Check file-level override first
	fileKey := normalizeFilePath(appID, filePath)
	if synced, ok := m.SyncedFiles[fileKey]; ok {
		return synced
	}
	// Also try with just the filePath
	if synced, ok := m.SyncedFiles[filePath]; ok {
		return synced
	}

	// Check app-level setting
	return m.IsAppSynced(appID)
}

// IsAppSynced returns true if the app has sync enabled
func (m *ModesConfig) IsAppSynced(appID string) bool {
	return m.SyncedApps[appID]
}

// ToggleAppSync toggles sync on/off for an app
func (m *ModesConfig) ToggleAppSync(appID string) bool {
	current := m.SyncedApps[appID]
	if current {
		delete(m.SyncedApps, appID)
	} else {
		m.SyncedApps[appID] = true
	}
	return !current
}

// ToggleFileSync toggles sync on/off for a specific file
func (m *ModesConfig) ToggleFileSync(appID, filePath string) bool {
	fileKey := normalizeFilePath(appID, filePath)
	current := m.SyncedFiles[fileKey]
	if current {
		delete(m.SyncedFiles, fileKey)
	} else {
		m.SyncedFiles[fileKey] = true
	}
	return !current
}

// SyncLabel returns "B" or "B+S" for UI display
func (m *ModesConfig) SyncLabel(appID, filePath string) string {
	if m.IsSynced(appID, filePath) {
		return "B+S"
	}
	return "B"
}

// AppSyncLabel returns "B" or "B+S" for app-level UI display
func (m *ModesConfig) AppSyncLabel(appID string) string {
	if m.IsAppSynced(appID) {
		return "B+S"
	}
	return "B"
}

// GetBackupPath returns the storage path for a backup (per-machine) file
// Format: dotfiles/{app}/{machine}/{file}
func (m *ModesConfig) GetBackupPath(basePath, appID, filePath string) string {
	filename := filepath.Base(filePath)
	return filepath.Join(basePath, appID, m.MachineName, filename)
}

// GetSyncPath returns the storage path for a shared (sync) file
// Format: dotfiles/{app}/{file}
func (m *ModesConfig) GetSyncPath(basePath, appID, filePath string) string {
	filename := filepath.Base(filePath)
	return filepath.Join(basePath, appID, filename)
}

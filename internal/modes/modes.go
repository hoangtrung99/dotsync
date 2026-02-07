package modes

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Mode represents the sync mode for an app or file
type Mode string

const (
	// ModeSync indicates files should be synced across all machines
	ModeSync Mode = "sync"
	// ModeBackup indicates files should be backed up per-machine
	ModeBackup Mode = "backup"
)

// ModesConfig holds the mode configuration for apps and files
type ModesConfig struct {
	MachineName string          `json:"machine_name"`
	DefaultMode Mode            `json:"default_mode"`
	Apps        map[string]Mode `json:"apps"`
	Files       map[string]Mode `json:"files"`
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
		MachineName: hostname,
		DefaultMode: ModeBackup,
		Apps:        make(map[string]Mode),
		Files:       make(map[string]Mode),
	}
}

// Load loads the modes configuration from file
func Load() (*ModesConfig, error) {
	configPath := ConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// First run - return default config
			return Default(), nil
		}
		return nil, err
	}

	var cfg ModesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Ensure maps are initialized
	if cfg.Apps == nil {
		cfg.Apps = make(map[string]Mode)
	}
	if cfg.Files == nil {
		cfg.Files = make(map[string]Mode)
	}

	// Use default if not set
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = ModeBackup
	}

	return &cfg, nil
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

// SetAppMode sets the mode for an app
func (m *ModesConfig) SetAppMode(appID string, mode Mode) {
	m.Apps[appID] = mode
}

// SetFileMode sets the mode for a specific file
func (m *ModesConfig) SetFileMode(filePath string, mode Mode) {
	m.Files[filePath] = mode
}

// RemoveAppMode removes the mode override for an app
func (m *ModesConfig) RemoveAppMode(appID string) {
	delete(m.Apps, appID)
}

// RemoveFileMode removes the mode override for a file
func (m *ModesConfig) RemoveFileMode(filePath string) {
	delete(m.Files, filePath)
}

// IsSync returns true if the mode is sync
func (mode Mode) IsSync() bool {
	return mode == ModeSync
}

// IsBackup returns true if the mode is backup
func (mode Mode) IsBackup() bool {
	return mode == ModeBackup
}

// String returns the string representation of the mode
func (mode Mode) String() string {
	return string(mode)
}

// Short returns a short indicator for UI display
func (mode Mode) Short() string {
	if mode == ModeSync {
		return "S"
	}
	return "B"
}

// Toggle returns the opposite mode
func (mode Mode) Toggle() Mode {
	if mode == ModeSync {
		return ModeBackup
	}
	return ModeSync
}

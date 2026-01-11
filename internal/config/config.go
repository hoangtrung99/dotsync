package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	DotfilesPath string `json:"dotfiles_path"` // Path to dotfiles directory
	BackupPath   string `json:"backup_path"`   // Path for backups
	AppsConfig   string `json:"apps_config"`   // Path to apps.yaml (optional)
	FirstRun     bool   `json:"-"`             // Is this the first run?
}

// configFileName is the name of the config file
const configFileName = "dotsync.json"

// Default returns the default configuration
func Default() *Config {
	homeDir, _ := os.UserHomeDir()

	return &Config{
		DotfilesPath: filepath.Join(homeDir, "dotfiles"),
		BackupPath:   filepath.Join(homeDir, ".dotfiles-backup"),
		AppsConfig:   "", // Empty = use built-in definitions
		FirstRun:     true,
	}
}

// ConfigPath returns the path to the config file
func ConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "dotsync", configFileName)
}

// Load loads the configuration from file
func Load() (*Config, error) {
	configPath := ConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// First run - return default config
			cfg := Default()
			cfg.FirstRun = true
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.FirstRun = false
	return &cfg, nil
}

// Save saves the configuration to file
func (c *Config) Save() error {
	configPath := ConfigPath()

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// EnsureDirectories creates necessary directories
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.DotfilesPath,
		c.BackupPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// GetDestPath returns the destination path in dotfiles for a given app
func (c *Config) GetDestPath(appID string) string {
	return filepath.Join(c.DotfilesPath, appID)
}

// GetBackupPath returns the backup path for a given file
func (c *Config) GetBackupPath(filename string) string {
	return filepath.Join(c.BackupPath, filename)
}

// DotfilesExists checks if dotfiles directory exists
func (c *Config) DotfilesExists() bool {
	_, err := os.Stat(c.DotfilesPath)
	return err == nil
}

// IsGitRepo checks if dotfiles is a git repository
func (c *Config) IsGitRepo() bool {
	gitPath := filepath.Join(c.DotfilesPath, ".git")
	_, err := os.Stat(gitPath)
	return err == nil
}

// SuggestedPaths returns suggested dotfiles paths
func SuggestedPaths() []string {
	homeDir, _ := os.UserHomeDir()
	return []string{
		filepath.Join(homeDir, "dotfiles"),
		filepath.Join(homeDir, ".dotfiles"),
		filepath.Join(homeDir, "Documents", "dotfiles"),
	}
}

// ConfigDir returns the directory containing dotsync config files
func ConfigDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "dotsync")
}

// StatePath returns the path to the sync state file
func (c *Config) StatePath() string {
	return filepath.Join(ConfigDir(), "sync_state.json")
}

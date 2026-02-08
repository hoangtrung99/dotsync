package customapps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dotsync/internal/models"

	"gopkg.in/yaml.v3"
)

// Store persists custom app definitions in a YAML file.
type Store struct {
	path string
}

// New creates a new custom app definition store.
func New(path string) *Store {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}
	return &Store{path: path}
}

// DefaultPath returns the default custom apps definition path.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dotsync", "apps.yaml")
}

// Load returns all custom app definitions.
func (s *Store) Load() ([]models.AppDefinition, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.AppDefinition{}, nil
		}
		return nil, err
	}

	var cfg models.AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Apps == nil {
		return []models.AppDefinition{}, nil
	}
	return cfg.Apps, nil
}

// Add appends a definition to the store.
func (s *Store) Add(def models.AppDefinition) error {
	def, err := sanitizeDefinition(def)
	if err != nil {
		return err
	}

	existing, err := s.Load()
	if err != nil {
		return err
	}

	for _, d := range existing {
		if strings.EqualFold(d.ID, def.ID) {
			return fmt.Errorf("custom app with id %q already exists", def.ID)
		}
	}

	existing = append(existing, def)
	return s.save(existing)
}

func (s *Store) save(defs []models.AppDefinition) error {
	cfg := models.AppConfig{Apps: defs}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func sanitizeDefinition(def models.AppDefinition) (models.AppDefinition, error) {
	def.ID = strings.TrimSpace(def.ID)
	def.Name = strings.TrimSpace(def.Name)
	def.Category = strings.TrimSpace(def.Category)
	def.Icon = strings.TrimSpace(def.Icon)

	if def.ID == "" {
		return def, fmt.Errorf("id is required")
	}
	if def.Name == "" {
		return def, fmt.Errorf("name is required")
	}
	if def.Category == "" {
		def.Category = "custom"
	}
	if def.Icon == "" {
		def.Icon = "📁"
	}

	cleaned := make([]string, 0, len(def.ConfigPaths))
	for _, p := range def.ConfigPaths {
		cp := normalizePath(p)
		if cp != "" {
			cleaned = append(cleaned, cp)
		}
	}
	if len(cleaned) == 0 {
		return def, fmt.Errorf("config_paths is required")
	}
	def.ConfigPaths = cleaned

	return def, nil
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.ToSlash(filepath.Clean(path))
	}

	if filepath.IsAbs(path) {
		home, _ := os.UserHomeDir()
		home = filepath.Clean(home)
		cleaned := filepath.Clean(path)
		if cleaned == home {
			return "~"
		}
		prefix := home + string(os.PathSeparator)
		if strings.HasPrefix(cleaned, prefix) {
			rel := strings.TrimPrefix(cleaned, prefix)
			return filepath.ToSlash(filepath.Join("~", rel))
		}
		return filepath.ToSlash(cleaned)
	}

	return filepath.ToSlash(filepath.Clean(path))
}

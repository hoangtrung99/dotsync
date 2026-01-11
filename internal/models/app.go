package models

// App represents a detected application with its config files
type App struct {
	ID          string   // Unique identifier
	Name        string   // Display name
	Category    string   // Category (terminal, editor, shell, etc.)
	Icon        string   // Emoji/nerd font icon
	ConfigPaths []string // Paths to check for existence
	Files       []File   // Detected config files
	Selected    bool     // Whether app is selected for sync
	Installed   bool     // Whether app is detected on system
}

// Category represents a group of apps
type Category struct {
	ID    string
	Name  string
	Icon  string
	Apps  []*App
	Count int // Number of installed apps
}

// AppDefinition is the YAML structure for app definitions
type AppDefinition struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	Category       string   `yaml:"category"`
	Icon           string   `yaml:"icon"`
	ConfigPaths    []string `yaml:"config_paths"`
	EncryptedFiles []string `yaml:"encrypted_files"`
}

// AppConfig is the root YAML structure
type AppConfig struct {
	Apps []AppDefinition `yaml:"apps"`
}

// NewApp creates a new App from definition
func NewApp(def AppDefinition) *App {
	return &App{
		ID:          def.ID,
		Name:        def.Name,
		Category:    def.Category,
		Icon:        def.Icon,
		ConfigPaths: def.ConfigPaths,
		Files:       []File{},
		Selected:    false,
		Installed:   false,
	}
}

// ToggleSelected toggles the selection state
func (a *App) ToggleSelected() {
	a.Selected = !a.Selected
}

// SelectAllFiles selects all files in the app
func (a *App) SelectAllFiles() {
	for i := range a.Files {
		a.Files[i].Selected = true
	}
}

// DeselectAllFiles deselects all files in the app
func (a *App) DeselectAllFiles() {
	for i := range a.Files {
		a.Files[i].Selected = false
	}
}

// SelectedFiles returns only selected files
func (a *App) SelectedFiles() []File {
	var selected []File
	for _, f := range a.Files {
		if f.Selected {
			selected = append(selected, f)
		}
	}
	return selected
}

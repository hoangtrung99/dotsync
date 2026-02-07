// Package editor provides IDE integration for merge and diff operations.
package editor

import (
	"fmt"
	"os/exec"
)

// Editor interface defines operations for IDE integration
type Editor interface {
	// Name returns the display name of the editor
	Name() string

	// IsInstalled checks if the editor is available on the system
	IsInstalled() bool

	// OpenMerge opens a 3-way merge view
	// local: path to local version
	// remote: path to remote version
	// merged: path to output merged file
	OpenMerge(local, remote, merged string) error

	// OpenDiff opens a diff view between two files
	OpenDiff(file1, file2 string) error

	// Wait blocks until the editor is closed
	Wait() error
}

// Config holds editor configuration
type Config struct {
	// Editor specifies which editor to use: "auto", "code", "cursor", "zed"
	Editor string `json:"editor"`

	// Priority order for auto-detection
	Priority []string `json:"editor_priority"`
}

// DefaultConfig returns the default editor configuration
func DefaultConfig() *Config {
	return &Config{
		Editor:   "auto",
		Priority: []string{"cursor", "code", "zed"},
	}
}

// availableEditors is the list of all supported editors
var availableEditors = []func() Editor{
	NewCursor,
	NewVSCode,
	NewZed,
}

// editorsByName maps editor names to constructor functions
var editorsByName = map[string]func() Editor{
	"code":   NewVSCode,
	"cursor": NewCursor,
	"zed":    NewZed,
}

// Detect finds an installed editor based on priority order
func Detect(cfg *Config) (Editor, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// If specific editor is requested, try that first
	if cfg.Editor != "" && cfg.Editor != "auto" {
		if constructor, ok := editorsByName[cfg.Editor]; ok {
			editor := constructor()
			if editor.IsInstalled() {
				return editor, nil
			}
			return nil, fmt.Errorf("editor %s is not installed", cfg.Editor)
		}
		return nil, fmt.Errorf("unknown editor: %s", cfg.Editor)
	}

	// Auto-detect based on priority
	priority := cfg.Priority
	if len(priority) == 0 {
		priority = DefaultConfig().Priority
	}

	// Try editors in priority order
	for _, name := range priority {
		if constructor, ok := editorsByName[name]; ok {
			editor := constructor()
			if editor.IsInstalled() {
				return editor, nil
			}
		}
	}

	// Fallback: try any available editor
	for _, constructor := range availableEditors {
		editor := constructor()
		if editor.IsInstalled() {
			return editor, nil
		}
	}

	return nil, fmt.Errorf("no supported editor found (install VS Code, Cursor, or Zed)")
}

// ListInstalled returns all installed editors
func ListInstalled() []Editor {
	var installed []Editor
	for _, constructor := range availableEditors {
		editor := constructor()
		if editor.IsInstalled() {
			installed = append(installed, editor)
		}
	}
	return installed
}

// isCommandAvailable checks if a command exists in PATH
func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// baseEditor provides common functionality for editors
type baseEditor struct {
	name    string
	command string
	cmd     *exec.Cmd
}

func (e *baseEditor) Name() string {
	return e.name
}

func (e *baseEditor) IsInstalled() bool {
	return isCommandAvailable(e.command)
}

func (e *baseEditor) Wait() error {
	if e.cmd == nil {
		return nil
	}
	return e.cmd.Wait()
}

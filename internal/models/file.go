package models

import (
	"os"
	"path/filepath"
	"time"
)

// File represents a config file that can be synced
type File struct {
	Name         string       // File name
	Path         string       // Full path on system
	RelPath      string       // Relative path for display
	Size         int64        // File size in bytes
	ModTime      time.Time    // Last modification time
	IsDir        bool         // Whether it's a directory
	Encrypted    bool         // Whether file should be encrypted
	Selected     bool         // Whether file is selected for sync
	SyncStatus   SyncStatus   // Sync status based on ModTime
	LocalHash    string       // SHA256 hash of local file
	DotfilesHash string       // SHA256 hash of dotfiles version
	ConflictType ConflictType // Conflict status based on hash comparison
}

// ConflictType represents the type of sync conflict
type ConflictType int

const (
	ConflictNone             ConflictType = iota
	ConflictLocalModified                 // Only local changed since last sync
	ConflictDotfilesModified              // Only dotfiles changed since last sync
	ConflictBothModified                  // Both changed - needs merge
	ConflictLocalNew                      // New in local only
	ConflictDotfilesNew                   // New in dotfiles only
	ConflictLocalDeleted                  // Deleted in local
	ConflictDotfilesDeleted               // Deleted in dotfiles
)

// ConflictIcon returns an icon for the conflict type
func (c ConflictType) ConflictIcon() string {
	switch c {
	case ConflictNone:
		return "✓"
	case ConflictLocalModified:
		return "●"
	case ConflictDotfilesModified:
		return "○"
	case ConflictBothModified:
		return "⚡"
	case ConflictLocalNew:
		return "+"
	case ConflictDotfilesNew:
		return "↓"
	case ConflictLocalDeleted, ConflictDotfilesDeleted:
		return "✗"
	default:
		return "?"
	}
}

// ConflictString returns a string description of the conflict
func (c ConflictType) ConflictString() string {
	switch c {
	case ConflictNone:
		return "Synced"
	case ConflictLocalModified:
		return "Modified (push)"
	case ConflictDotfilesModified:
		return "Outdated (pull)"
	case ConflictBothModified:
		return "CONFLICT"
	case ConflictLocalNew:
		return "New (local)"
	case ConflictDotfilesNew:
		return "New (dotfiles)"
	case ConflictLocalDeleted:
		return "Deleted locally"
	case ConflictDotfilesDeleted:
		return "Deleted in dotfiles"
	default:
		return "Unknown"
	}
}

// SyncStatus represents the sync state of a file
type SyncStatus int

const (
	StatusUnknown  SyncStatus = iota
	StatusSynced              // File is in sync
	StatusModified            // Local file is newer
	StatusOutdated            // Dotfiles version is newer
	StatusNew                 // File doesn't exist in dotfiles
	StatusMissing             // File doesn't exist locally
)

// StatusIcon returns an icon representing the sync status
func (s SyncStatus) StatusIcon() string {
	switch s {
	case StatusSynced:
		return "✓"
	case StatusModified:
		return "●"
	case StatusOutdated:
		return "○"
	case StatusNew:
		return "+"
	case StatusMissing:
		return "✗"
	default:
		return "?"
	}
}

// String returns a string representation of the status
func (s SyncStatus) String() string {
	switch s {
	case StatusSynced:
		return "Synced"
	case StatusModified:
		return "Modified"
	case StatusOutdated:
		return "Outdated"
	case StatusNew:
		return "New"
	case StatusMissing:
		return "Missing"
	default:
		return "Unknown"
	}
}

// NewFile creates a File from a path
func NewFile(path string, basePath string) (*File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(basePath, path)
	if relPath == "" {
		relPath = filepath.Base(path)
	}

	return &File{
		Name:       filepath.Base(path),
		Path:       path,
		RelPath:    relPath,
		Size:       info.Size(),
		ModTime:    info.ModTime(),
		IsDir:      info.IsDir(),
		Selected:   true, // Default to selected
		SyncStatus: StatusUnknown,
	}, nil
}

// ToggleSelected toggles the selection state
func (f *File) ToggleSelected() {
	f.Selected = !f.Selected
}

// SizeHuman returns human-readable file size
func (f *File) SizeHuman() string {
	const unit = 1024
	if f.Size < unit {
		return formatSize(f.Size, "B")
	}
	div, exp := int64(unit), 0
	for n := f.Size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return formatSize(f.Size/div, []string{"KB", "MB", "GB", "TB"}[exp])
}

func formatSize(size int64, unit string) string {
	if size == 0 {
		return "0 " + unit
	}
	return string(rune('0'+size%10)) + " " + unit
}

// Icon returns an icon based on file type
func (f *File) Icon() string {
	if f.IsDir {
		return "📁"
	}

	ext := filepath.Ext(f.Name)
	switch ext {
	case ".json":
		return "📋"
	case ".yaml", ".yml":
		return "📄"
	case ".toml":
		return "⚙️"
	case ".lua":
		return "🌙"
	case ".sh", ".bash", ".zsh", ".fish":
		return "🐚"
	case ".conf", ".config":
		return "🔧"
	default:
		return "📄"
	}
}

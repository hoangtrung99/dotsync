// Package suggestions provides smart action suggestions based on sync state.
package suggestions

import (
	"fmt"

	"dotsync/internal/modes"
)

// SuggestionType represents the type of suggestion
type SuggestionType int

const (
	// TypeAllSynced indicates everything is in sync
	TypeAllSynced SuggestionType = iota
	// TypeLocalModified indicates local files have been modified
	TypeLocalModified
	// TypeRemoteUpdated indicates remote has new updates
	TypeRemoteUpdated
	// TypeConflicts indicates there are conflicts to resolve
	TypeConflicts
	// TypeFirstRun indicates this is the first run
	TypeFirstRun
)

// String returns the string representation of the suggestion type
func (t SuggestionType) String() string {
	switch t {
	case TypeAllSynced:
		return "all_synced"
	case TypeLocalModified:
		return "local_modified"
	case TypeRemoteUpdated:
		return "remote_updated"
	case TypeConflicts:
		return "conflicts"
	case TypeFirstRun:
		return "first_run"
	default:
		return "unknown"
	}
}

// Action represents a suggested action
type Action struct {
	Key         string // Keyboard shortcut (e.g., "P", "Q")
	Label       string // Display label (e.g., "Push now")
	Description string // Longer description
}

// Suggestion represents a smart suggestion for the user
type Suggestion struct {
	Type    SuggestionType
	Message string   // Main message to display
	Actions []Action // Available actions
	Files   []string // Affected files
	Count   int      // Number of affected items
}

// Icon returns an appropriate icon for the suggestion type
func (s *Suggestion) Icon() string {
	switch s.Type {
	case TypeAllSynced:
		return "[OK]"
	case TypeLocalModified:
		return "[UP]"
	case TypeRemoteUpdated:
		return "[DN]"
	case TypeConflicts:
		return "[!!]"
	case TypeFirstRun:
		return "[HI]"
	default:
		return "[--]"
	}
}

// IsEmpty returns true if there's no actionable suggestion
func (s *Suggestion) IsEmpty() bool {
	return s.Type == TypeAllSynced && len(s.Files) == 0
}

// FileState represents the sync state of a single file
type FileState struct {
	Path           string
	LocalModified  bool
	RemoteModified bool
	HasConflict    bool
	Synced         bool
}

// SyncState represents the overall sync state
type SyncState struct {
	Files         []FileState
	HasLocalRepo  bool
	HasRemote     bool
	IsFirstRun    bool
	LocalAhead    int // commits ahead of remote
	LocalBehind   int // commits behind remote
}

// Analyzer analyzes sync state and generates suggestions
type Analyzer struct {
	modesConfig *modes.ModesConfig
}

// NewAnalyzer creates a new suggestion analyzer
func NewAnalyzer(modesCfg *modes.ModesConfig) *Analyzer {
	return &Analyzer{
		modesConfig: modesCfg,
	}
}

// AnalyzeState analyzes the current sync state and returns an appropriate suggestion
func (a *Analyzer) AnalyzeState(state *SyncState) *Suggestion {
	// First run check
	if state.IsFirstRun {
		return a.firstRunSuggestion()
	}

	// Count files in different states
	var localModified, remoteUpdated, conflicts []string

	for _, f := range state.Files {
		if f.HasConflict {
			conflicts = append(conflicts, f.Path)
		} else if f.LocalModified {
			localModified = append(localModified, f.Path)
		} else if f.RemoteModified {
			remoteUpdated = append(remoteUpdated, f.Path)
		}
	}

	// Priority: Conflicts > Local Modified > Remote Updated > All Synced
	if len(conflicts) > 0 {
		return a.conflictsSuggestion(conflicts)
	}

	if len(localModified) > 0 {
		return a.localModifiedSuggestion(localModified)
	}

	if len(remoteUpdated) > 0 {
		return a.remoteUpdatedSuggestion(remoteUpdated)
	}

	return a.allSyncedSuggestion()
}

// firstRunSuggestion creates a suggestion for first-time users
func (a *Analyzer) firstRunSuggestion() *Suggestion {
	return &Suggestion{
		Type:    TypeFirstRun,
		Message: "Welcome! Select apps to sync",
		Count:   0,
		Actions: []Action{
			{Key: "A", Label: "Select all", Description: "Select all detected apps"},
			{Key: "Enter", Label: "Configure", Description: "Configure selected apps"},
			{Key: "?", Label: "Help", Description: "View help"},
		},
	}
}

// conflictsSuggestion creates a suggestion for conflict resolution
func (a *Analyzer) conflictsSuggestion(files []string) *Suggestion {
	msg := fmt.Sprintf("%d conflicts detected", len(files))
	if len(files) == 1 {
		msg = "1 conflict detected"
	}

	return &Suggestion{
		Type:    TypeConflicts,
		Message: msg,
		Files:   files,
		Count:   len(files),
		Actions: []Action{
			{Key: "Q", Label: "Quick backup", Description: "Resolve conflicts in editor"},
			{Key: "E", Label: "Open editor", Description: "Open files in editor"},
			{Key: "D", Label: "View diff", Description: "View differences"},
		},
	}
}

// localModifiedSuggestion creates a suggestion for local modifications
func (a *Analyzer) localModifiedSuggestion(files []string) *Suggestion {
	msg := fmt.Sprintf("%d files modified locally", len(files))
	if len(files) == 1 {
		msg = "1 file modified locally"
	}

	return &Suggestion{
		Type:    TypeLocalModified,
		Message: msg,
		Files:   files,
		Count:   len(files),
		Actions: []Action{
			{Key: "P", Label: "Push now", Description: "Push changes to dotfiles repo"},
			{Key: "Q", Label: "Quick backup", Description: "Sync with remote"},
			{Key: "D", Label: "Details", Description: "View changed files"},
		},
	}
}

// remoteUpdatedSuggestion creates a suggestion for remote updates
func (a *Analyzer) remoteUpdatedSuggestion(files []string) *Suggestion {
	msg := fmt.Sprintf("%d updates available", len(files))
	if len(files) == 1 {
		msg = "1 update available"
	}

	return &Suggestion{
		Type:    TypeRemoteUpdated,
		Message: msg,
		Files:   files,
		Count:   len(files),
		Actions: []Action{
			{Key: "L", Label: "Pull", Description: "Pull updates from remote"},
			{Key: "Q", Label: "Quick backup", Description: "Sync with remote"},
			{Key: "D", Label: "Details", Description: "View updated files"},
		},
	}
}

// allSyncedSuggestion creates a suggestion when everything is synced
func (a *Analyzer) allSyncedSuggestion() *Suggestion {
	return &Suggestion{
		Type:    TypeAllSynced,
		Message: "Everything synced",
		Count:   0,
		Actions: []Action{
			{Key: "R", Label: "Refresh", Description: "Check for updates"},
			{Key: "G", Label: "Git", Description: "Open git panel"},
		},
	}
}

// AnalyzeFiles is a convenience function that analyzes a list of file states
func AnalyzeFiles(files []FileState, isFirstRun bool) *Suggestion {
	modesCfg, _ := modes.Load()
	analyzer := NewAnalyzer(modesCfg)

	state := &SyncState{
		Files:      files,
		IsFirstRun: isFirstRun,
	}

	return analyzer.AnalyzeState(state)
}

// QuickAnalyze creates a quick suggestion based on simple counts
func QuickAnalyze(localModified, remoteUpdated, conflicts int) *Suggestion {
	if conflicts > 0 {
		msg := fmt.Sprintf("%d conflicts detected", conflicts)
		if conflicts == 1 {
			msg = "1 conflict detected"
		}
		return &Suggestion{
			Type:    TypeConflicts,
			Message: msg,
			Count:   conflicts,
			Actions: []Action{
				{Key: "Q", Label: "Quick backup", Description: "Resolve conflicts in editor"},
			},
		}
	}

	if localModified > 0 {
		msg := fmt.Sprintf("%d files modified locally", localModified)
		if localModified == 1 {
			msg = "1 file modified locally"
		}
		return &Suggestion{
			Type:    TypeLocalModified,
			Message: msg,
			Count:   localModified,
			Actions: []Action{
				{Key: "P", Label: "Push now", Description: "Push changes to dotfiles repo"},
				{Key: "Q", Label: "Quick backup", Description: "Sync with remote"},
			},
		}
	}

	if remoteUpdated > 0 {
		msg := fmt.Sprintf("%d updates available", remoteUpdated)
		if remoteUpdated == 1 {
			msg = "1 update available"
		}
		return &Suggestion{
			Type:    TypeRemoteUpdated,
			Message: msg,
			Count:   remoteUpdated,
			Actions: []Action{
				{Key: "L", Label: "Pull", Description: "Pull updates from remote"},
				{Key: "Q", Label: "Quick backup", Description: "Sync with remote"},
			},
		}
	}

	return &Suggestion{
		Type:    TypeAllSynced,
		Message: "Everything synced",
		Count:   0,
	}
}

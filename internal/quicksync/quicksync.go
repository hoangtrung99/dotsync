// Package quicksync provides fast sync workflow for dotfiles.
// It orchestrates git fetch, state detection, and auto-resolution.
package quicksync

import (
	"fmt"

	"dotsync/internal/backup"
	"dotsync/internal/config"
	"dotsync/internal/editor"
	"dotsync/internal/git"
	"dotsync/internal/modes"
	"dotsync/internal/models"
)

// ActionType represents the overall action taken
type ActionType int

const (
	// ActionSynced - everything was already synced
	ActionSynced ActionType = iota
	// ActionBackedUp - backup files were auto-pushed
	ActionBackedUp
	// ActionPulled - remote changes were pulled
	ActionPulled
	// ActionMerged - conflicts were merged
	ActionMerged
	// ActionPending - sync files need manual action
	ActionPending
	// ActionFailed - an error occurred
	ActionFailed
)

// String returns a human-readable string for the action
func (a ActionType) String() string {
	switch a {
	case ActionSynced:
		return "synced"
	case ActionBackedUp:
		return "backed up"
	case ActionPulled:
		return "pulled"
	case ActionMerged:
		return "merged"
	case ActionPending:
		return "pending"
	case ActionFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Result contains the result of a Quick Sync operation
type Result struct {
	// Overall action taken
	Action ActionType

	// Backup mode results
	BackedUpCount int
	BackupFiles   []FileInfo

	// Sync mode status (for manual action)
	SyncLocalMod  int
	SyncRemoteMod int
	SyncConflicts int
	SyncFiles     []FileInfo // Sync files that need manual action

	// Git operations
	Fetched       bool
	Committed     bool
	CommitMessage string

	// Error if any
	Error error

	// Detection result for detailed info
	Detection *DetectionResult
}

// Summary returns a human-readable summary of the result
func (r *Result) Summary() string {
	switch r.Action {
	case ActionSynced:
		return "All files are in sync"
	case ActionBackedUp:
		return fmt.Sprintf("Backed up %d files", r.BackedUpCount)
	case ActionPending:
		return r.formatPendingMessage()
	case ActionFailed:
		return fmt.Sprintf("Error: %v", r.Error)
	default:
		return r.Action.String()
	}
}

// formatPendingMessage formats the message for pending sync files
func (r *Result) formatPendingMessage() string {
	parts := []string{}

	if r.BackedUpCount > 0 {
		parts = append(parts, fmt.Sprintf("Backed up: %d files", r.BackedUpCount))
	}

	if r.SyncLocalMod > 0 {
		parts = append(parts, fmt.Sprintf("%d modified (push)", r.SyncLocalMod))
	}
	if r.SyncRemoteMod > 0 {
		parts = append(parts, fmt.Sprintf("%d outdated (pull)", r.SyncRemoteMod))
	}
	if r.SyncConflicts > 0 {
		parts = append(parts, fmt.Sprintf("%d conflicts", r.SyncConflicts))
	}

	if len(parts) == 0 {
		return "No changes"
	}

	msg := ""
	for i, p := range parts {
		if i > 0 {
			msg += "\n"
		}
		msg += p
	}
	return msg
}

// HasSyncPending returns true if there are sync files needing manual action
func (r *Result) HasSyncPending() bool {
	return r.SyncLocalMod > 0 || r.SyncRemoteMod > 0 || r.SyncConflicts > 0
}

// QuickSync orchestrates the quick sync workflow
type QuickSync struct {
	config        *config.Config
	modesConfig   *modes.ModesConfig
	gitRepo       *git.Repo
	detector      *ConflictDetector
	resolver      *Resolver
	backupManager *backup.BackupManager
	editorConfig  *editor.Config
}

// New creates a new QuickSync instance
func New(cfg *config.Config, modesCfg *modes.ModesConfig) *QuickSync {
	gitRepo := git.NewRepo(cfg.DotfilesPath)
	detector := NewConflictDetector(cfg, modesCfg)
	resolver := NewResolver(cfg, modesCfg, gitRepo, detector)
	backupMgr := backup.New(cfg, modesCfg)

	return &QuickSync{
		config:        cfg,
		modesConfig:   modesCfg,
		gitRepo:       gitRepo,
		detector:      detector,
		resolver:      resolver,
		backupManager: backupMgr,
		editorConfig:  editor.DefaultConfig(),
	}
}

// WithEditor sets a custom editor configuration
func (q *QuickSync) WithEditor(cfg *editor.Config) *QuickSync {
	q.editorConfig = cfg
	return q
}

// Run executes the Quick Sync workflow:
// 1. Git fetch (get updates from remote)
// 2. Detect state (compare local vs remote vs dotfiles)
// 3. Handle by mode:
//   - BACKUP files: auto-push to dotfiles/app/{machine}/
//   - SYNC files: only report status, don't auto-resolve
//
// Returns QuickSyncResult with what was done
func (q *QuickSync) Run(apps []*models.App) *Result {
	result := &Result{
		Action:      ActionSynced,
		BackupFiles: []FileInfo{},
		SyncFiles:   []FileInfo{},
	}

	// Step 1: Git fetch
	if q.gitRepo != nil && q.gitRepo.IsRepo() && q.gitRepo.HasRemote() {
		if err := q.gitRepo.Fetch(); err != nil {
			// Fetch failed - continue anyway, might be offline
			// result.Error = fmt.Errorf("fetch failed: %w", err)
		} else {
			result.Fetched = true
		}
	}

	// Step 2: Detect state
	detection := q.detector.DetectAll(apps)
	result.Detection = detection

	// Step 3: Handle by mode

	// 3a. Handle BACKUP files (auto-push)
	resolveResult := q.resolver.ResolveAuto(detection)

	// Count successful backups
	for _, res := range resolveResult.BackupResults {
		if res.Action == ActionPush && res.Error == nil {
			result.BackedUpCount++
			result.BackupFiles = append(result.BackupFiles, res.File)
		}
	}

	result.Committed = resolveResult.Committed
	result.CommitMessage = resolveResult.CommitMessage

	// 3b. Collect SYNC files status
	result.SyncFiles = resolveResult.SyncFiles

	for _, f := range result.SyncFiles {
		switch f.State {
		case StateLocalModified, StateLocalNew:
			result.SyncLocalMod++
		case StateRemoteModified, StateRemoteNew:
			result.SyncRemoteMod++
		case StateConflict:
			result.SyncConflicts++
		}
	}

	// Determine overall action
	if resolveResult.Error != nil {
		result.Action = ActionFailed
		result.Error = resolveResult.Error
	} else if result.BackedUpCount > 0 && !result.HasSyncPending() {
		result.Action = ActionBackedUp
	} else if result.HasSyncPending() {
		result.Action = ActionPending
	} else {
		result.Action = ActionSynced
	}

	return result
}

// GetDetector returns the conflict detector
func (q *QuickSync) GetDetector() *ConflictDetector {
	return q.detector
}

// GetResolver returns the resolver
func (q *QuickSync) GetResolver() *Resolver {
	return q.resolver
}

// GetGitRepo returns the git repository
func (q *QuickSync) GetGitRepo() *git.Repo {
	return q.gitRepo
}

// DetectOnly runs detection without auto-resolving
func (q *QuickSync) DetectOnly(apps []*models.App) *DetectionResult {
	return q.detector.DetectAll(apps)
}

// Push pushes changes to git remote
func (q *QuickSync) Push() error {
	if q.gitRepo == nil || !q.gitRepo.IsRepo() {
		return fmt.Errorf("not a git repository")
	}
	return q.gitRepo.Push()
}

// Pull pulls changes from git remote
func (q *QuickSync) Pull() error {
	if q.gitRepo == nil || !q.gitRepo.IsRepo() {
		return fmt.Errorf("not a git repository")
	}
	return q.gitRepo.Pull()
}

// OpenConflictInEditor opens conflict files in the configured editor
func (q *QuickSync) OpenConflictInEditor(file FileInfo) error {
	ed, err := editor.Detect(q.editorConfig)
	if err != nil {
		return err
	}

	// Open merge view with local, remote, and merged files
	return ed.OpenMerge(file.FilePath, file.DotfilesPath, file.FilePath)
}

// PushFile pushes a single file to dotfiles
func (q *QuickSync) PushFile(file FileInfo) error {
	// Push the file
	if err := q.resolver.pushFile(file); err != nil {
		return err
	}

	// Update sync state
	if err := q.resolver.UpdateSyncState(file); err != nil {
		return err
	}

	// Save state
	return q.detector.SaveState()
}

// PullFile pulls a single file from dotfiles
func (q *QuickSync) PullFile(file FileInfo) error {
	// Pull the file
	if err := q.resolver.pullFile(file); err != nil {
		return err
	}

	// Update sync state
	if err := q.resolver.UpdateSyncState(file); err != nil {
		return err
	}

	// Save state
	return q.detector.SaveState()
}

// CommitAndPush commits changes and pushes to remote
func (q *QuickSync) CommitAndPush(message string) error {
	if q.gitRepo == nil || !q.gitRepo.IsRepo() {
		return fmt.Errorf("not a git repository")
	}

	// Add all changes
	if err := q.gitRepo.AddAll(); err != nil {
		return fmt.Errorf("add failed: %w", err)
	}

	// Commit
	if err := q.gitRepo.Commit(message); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	// Push
	if q.gitRepo.HasRemote() {
		if err := q.gitRepo.Push(); err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
	}

	return nil
}

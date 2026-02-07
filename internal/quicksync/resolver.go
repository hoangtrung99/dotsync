package quicksync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dotsync/internal/config"
	"dotsync/internal/git"
	"dotsync/internal/modes"
	"dotsync/internal/sync"
)

// ResolveAction represents the action to take for a file
type ResolveAction int

const (
	// ActionNone - no action needed
	ActionNone ResolveAction = iota
	// ActionPush - push local to dotfiles
	ActionPush
	// ActionPull - pull dotfiles to local
	ActionPull
	// ActionMerge - needs manual merge
	ActionMerge
	// ActionSkip - skip this file
	ActionSkip
)

// String returns a human-readable string for the action
func (a ResolveAction) String() string {
	switch a {
	case ActionNone:
		return "none"
	case ActionPush:
		return "push"
	case ActionPull:
		return "pull"
	case ActionMerge:
		return "merge"
	case ActionSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// ResolveResult contains the result of a resolve operation
type ResolveResult struct {
	File   FileInfo
	Action ResolveAction
	Error  error
}

// Resolver handles auto-resolution of file states
type Resolver struct {
	config      *config.Config
	modesConfig *modes.ModesConfig
	gitRepo     *git.Repo
	detector    *ConflictDetector
}

// NewResolver creates a new Resolver
func NewResolver(cfg *config.Config, modesCfg *modes.ModesConfig, gitRepo *git.Repo, detector *ConflictDetector) *Resolver {
	return &Resolver{
		config:      cfg,
		modesConfig: modesCfg,
		gitRepo:     gitRepo,
		detector:    detector,
	}
}

// DetermineAction determines what action to take for a file based on its state
// In the new model, all files are always backed up. Synced files also get pushed to shared path.
func (r *Resolver) DetermineAction(file FileInfo) ResolveAction {
	switch file.State {
	case StateLocalModified, StateLocalNew:
		return ActionPush
	case StateSynced:
		return ActionNone
	case StateConflict:
		if file.Synced {
			return ActionMerge
		}
		// For backup-only files, always prefer local (push)
		return ActionPush
	default:
		return ActionPush
	}
}

// ResolveBackupFiles resolves all files (auto-push to backup path)
func (r *Resolver) ResolveBackupFiles(files []FileInfo) []ResolveResult {
	var results []ResolveResult

	for _, file := range files {
		action := r.DetermineAction(file)
		result := ResolveResult{
			File:   file,
			Action: action,
		}

		if action == ActionPush {
			// Always push to backup path
			err := r.pushFile(file)
			result.Error = err

			// If synced, also push to shared path
			if err == nil && file.Synced && file.SyncPath != "" {
				syncFile := file
				syncFile.DotfilesPath = file.SyncPath
				if syncErr := r.pushFile(syncFile); syncErr != nil {
					result.Error = syncErr
				}
			}
		}

		results = append(results, result)
	}

	return results
}

// pushFile copies a file from local to dotfiles
func (r *Resolver) pushFile(file FileInfo) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(file.DotfilesPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if source is a directory
	srcInfo, err := os.Stat(file.FilePath)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return r.copyDir(file.FilePath, file.DotfilesPath)
	}

	return r.copyFile(file.FilePath, file.DotfilesPath)
}

// pullFile copies a file from dotfiles to local
func (r *Resolver) pullFile(file FileInfo) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(file.FilePath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if source is a directory
	srcInfo, err := os.Stat(file.DotfilesPath)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return r.copyDir(file.DotfilesPath, file.FilePath)
	}

	return r.copyFile(file.DotfilesPath, file.FilePath)
}

// copyFile copies a single file
func (r *Resolver) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	// Preserve permissions
	srcInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, srcInfo.Mode())
	}

	return nil
}

// copyDir copies a directory recursively
func (r *Resolver) copyDir(src, dst string) error {
	// Remove destination directory first
	os.RemoveAll(dst)

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return r.copyFile(path, dstPath)
	})
}

// UpdateSyncState updates the sync state after resolving
func (r *Resolver) UpdateSyncState(file FileInfo) error {
	// Compute new hashes after sync
	localHash, _ := sync.ComputeFileHash(file.FilePath)
	remoteHash, _ := sync.ComputeFileHash(file.DotfilesPath)

	// Update state manager using the same relPath key as detectFileState
	r.detector.UpdateFileState(file.AppID, file.RelPath, localHash, remoteHash)

	return nil
}

// CommitChanges creates a git commit for the changes
func (r *Resolver) CommitChanges(message string, files []FileInfo) error {
	if r.gitRepo == nil || !r.gitRepo.IsRepo() {
		return nil // No git repo, nothing to commit
	}

	// Add changed files
	for _, file := range files {
		relPath, err := filepath.Rel(r.config.DotfilesPath, file.DotfilesPath)
		if err != nil {
			continue
		}
		_ = r.gitRepo.Add(relPath)
	}

	// Commit
	return r.gitRepo.Commit(message)
}

// GenerateCommitMessage generates a commit message for the changes
func GenerateCommitMessage(files []FileInfo) string {
	if len(files) == 0 {
		return "sync: update configs"
	}

	// Get unique app IDs
	apps := make(map[string]bool)
	for _, f := range files {
		apps[f.AppID] = true
	}

	// Generate message
	if len(apps) == 1 {
		for appID := range apps {
			return fmt.Sprintf("sync: update %s (%d files)", appID, len(files))
		}
	}

	appList := ""
	i := 0
	for appID := range apps {
		if i > 0 {
			appList += ", "
		}
		appList += appID
		i++
		if i >= 3 {
			appList += fmt.Sprintf(" +%d more", len(apps)-3)
			break
		}
	}

	return fmt.Sprintf("sync: update %s", appList)
}

// ResolveAutoResult contains the result of auto-resolve
type ResolveAutoResult struct {
	BackupResults []ResolveResult
	SyncFiles     []FileInfo // Files that need manual action
	Committed     bool
	CommitMessage string
	Error         error
}

// ResolveAuto automatically resolves backup files and reports sync files
func (r *Resolver) ResolveAuto(detection *DetectionResult) *ResolveAutoResult {
	result := &ResolveAutoResult{
		BackupResults: []ResolveResult{},
		SyncFiles:     []FileInfo{},
	}

	// Get backup files with local changes
	backupFiles := detection.GetBackupFilesWithChanges()

	// Auto-resolve backup files
	if len(backupFiles) > 0 {
		result.BackupResults = r.ResolveBackupFiles(backupFiles)

		// Count successful pushes
		successfulPushes := []FileInfo{}
		for _, res := range result.BackupResults {
			if res.Action == ActionPush && res.Error == nil {
				successfulPushes = append(successfulPushes, res.File)
				// Update sync state
				_ = r.UpdateSyncState(res.File)
			}
		}

		// Commit if there were successful pushes
		// Use AddAll to stage everything (both backup and sync path files)
		// so all changes are captured in a single commit
		if len(successfulPushes) > 0 {
			result.CommitMessage = GenerateCommitMessage(successfulPushes)
			if err := r.gitRepo.AddAll(); err != nil {
				result.Error = fmt.Errorf("add failed: %w", err)
			} else if err := r.gitRepo.Commit(result.CommitMessage); err != nil {
				result.Error = fmt.Errorf("commit failed: %w", err)
			} else {
				result.Committed = true
			}
		}

		// Save sync state
		_ = r.detector.SaveState()
	}

	// Collect sync files that need manual action
	result.SyncFiles = detection.GetSyncFilesWithChanges()

	return result
}

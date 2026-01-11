package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"dotsync/internal/config"
	"dotsync/internal/models"
)

// Importer handles importing configs from dotfiles to system
type Importer struct {
	config *config.Config
}

// NewImporter creates a new Importer
func NewImporter(cfg *config.Config) *Importer {
	return &Importer{config: cfg}
}

// ImportResult holds the result of an import operation
type ImportResult struct {
	App        *models.App
	File       models.File
	Success    bool
	Error      error
	BackupPath string
}

// ImportApp imports all selected files for an app
func (i *Importer) ImportApp(app *models.App) ([]ImportResult, error) {
	var results []ImportResult

	srcDir := i.config.GetDestPath(app.ID)

	// Check if app directory exists in dotfiles
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return results, nil // Skip if no dotfiles for this app
	}

	for _, file := range app.Files {
		if !file.Selected {
			continue
		}

		result := ImportResult{
			App:  app,
			File: file,
		}

		srcPath := filepath.Join(srcDir, file.RelPath)
		dstPath := file.Path

		// Check if source exists in dotfiles
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			result.Error = fmt.Errorf("file not found in dotfiles: %s", srcPath)
			results = append(results, result)
			continue
		}

		// Create parent directory if not exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			result.Error = fmt.Errorf("failed to create directory: %w", err)
			results = append(results, result)
			continue
		}

		// Backup existing file if it exists
		if _, err := os.Stat(dstPath); err == nil {
			backupPath, err := Backup(dstPath, i.config.BackupPath)
			if err != nil {
				result.Error = fmt.Errorf("backup failed: %w", err)
				results = append(results, result)
				continue
			}
			result.BackupPath = backupPath
		}

		// Import the file
		exporter := &Exporter{}
		srcInfo, err := os.Stat(srcPath)
		if err != nil {
			result.Error = fmt.Errorf("cannot stat source: %w", err)
			results = append(results, result)
			continue
		}

		if srcInfo.IsDir() {
			// Remove existing directory first
			os.RemoveAll(dstPath)
			err = exporter.copyDir(srcPath, dstPath)
		} else {
			err = exporter.copyFile(srcPath, dstPath)
		}

		result.Success = err == nil
		result.Error = err
		results = append(results, result)
	}

	return results, nil
}

// ImportAll imports all selected apps and files
func (i *Importer) ImportAll(apps []*models.App) ([]ImportResult, error) {
	var allResults []ImportResult

	for _, app := range apps {
		if !app.Selected {
			continue
		}

		results, err := i.ImportApp(app)
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// CompareFiles compares local and dotfiles versions
func CompareFiles(localPath, dotfilesPath string) models.SyncStatus {
	localInfo, localErr := os.Stat(localPath)
	dotfilesInfo, dotfilesErr := os.Stat(dotfilesPath)

	if localErr != nil && dotfilesErr != nil {
		return models.StatusUnknown
	}

	if localErr != nil {
		return models.StatusMissing
	}

	if dotfilesErr != nil {
		return models.StatusNew
	}

	// Compare modification times
	if localInfo.ModTime().After(dotfilesInfo.ModTime()) {
		return models.StatusModified
	} else if dotfilesInfo.ModTime().After(localInfo.ModTime()) {
		return models.StatusOutdated
	}

	return models.StatusSynced
}

// UpdateSyncStatus updates the sync status for all files in an app
func UpdateSyncStatus(app *models.App, dotfilesPath string) {
	appDir := filepath.Join(dotfilesPath, app.ID)

	for i := range app.Files {
		file := &app.Files[i]
		dotfilesFilePath := filepath.Join(appDir, file.RelPath)
		file.SyncStatus = CompareFiles(file.Path, dotfilesFilePath)
	}
}

// UpdateSyncStatusWithHashes updates sync status with hash-based conflict detection
func UpdateSyncStatusWithHashes(app *models.App, dotfilesPath string, stateManager *StateManager) {
	appDir := filepath.Join(dotfilesPath, app.ID)

	for i := range app.Files {
		file := &app.Files[i]
		dotfilesFilePath := filepath.Join(appDir, file.RelPath)

		// Compute hashes
		localHash := ""
		dotfilesHash := ""

		if _, err := os.Stat(file.Path); err == nil {
			if file.IsDir {
				localHash, _ = ComputeDirHash(file.Path)
			} else {
				localHash, _ = ComputeFileHash(file.Path)
			}
		}

		if _, err := os.Stat(dotfilesFilePath); err == nil {
			if file.IsDir {
				dotfilesHash, _ = ComputeDirHash(dotfilesFilePath)
			} else {
				dotfilesHash, _ = ComputeFileHash(dotfilesFilePath)
			}
		}

		file.LocalHash = localHash
		file.DotfilesHash = dotfilesHash

		// Detect conflict using state manager
		if stateManager != nil {
			file.ConflictType = stateManager.DetectConflict(app.ID, file.RelPath, localHash, dotfilesHash)
		} else {
			// Fallback: simple hash comparison without history
			file.ConflictType = detectConflictSimple(localHash, dotfilesHash)
		}

		// Also update the legacy SyncStatus for backwards compatibility
		file.SyncStatus = CompareFiles(file.Path, dotfilesFilePath)
	}
}

// detectConflictSimple detects conflicts without sync state history
func detectConflictSimple(localHash, dotfilesHash string) models.ConflictType {
	if localHash == "" && dotfilesHash == "" {
		return models.ConflictNone
	}
	if localHash == "" {
		return models.ConflictDotfilesNew
	}
	if dotfilesHash == "" {
		return models.ConflictLocalNew
	}
	if localHash == dotfilesHash {
		return models.ConflictNone
	}
	// Both exist but different - without history, we can't tell who changed
	// so we mark it as both modified (needs user decision)
	return models.ConflictBothModified
}

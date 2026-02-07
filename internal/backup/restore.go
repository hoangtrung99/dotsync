package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RestoreResult contains the result of a restore operation
type RestoreResult struct {
	Restored     []RestoredFile
	BackedUpOld  []BackedUpFile
	Errors       []RestoreError
	SourceMachine string
}

// RestoredFile represents a successfully restored file
type RestoredFile struct {
	AppID      string
	FileName   string
	SourcePath string
	DestPath   string
	Size       int64
}

// RestoreError represents an error during restore
type RestoreError struct {
	AppID    string
	FileName string
	Error    error
}

// RestoreOptions configures the restore operation
type RestoreOptions struct {
	SourceMachine string   // Machine to restore from
	Files         []string // List of files to restore (appID/filename format)
	BackupCurrent bool     // Whether to backup current files before restoring
}

// Restore restores files from another machine's backup
func (b *BackupManager) Restore(opts RestoreOptions) (*RestoreResult, error) {
	result := &RestoreResult{
		Restored:      []RestoredFile{},
		BackedUpOld:   []BackedUpFile{},
		Errors:        []RestoreError{},
		SourceMachine: opts.SourceMachine,
	}

	// Validate source machine exists
	machines, err := b.ListMachines()
	if err != nil {
		return nil, fmt.Errorf("failed to list machines: %w", err)
	}

	found := false
	for _, m := range machines {
		if m.Name == opts.SourceMachine {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("source machine '%s' not found", opts.SourceMachine)
	}

	for _, fileSpec := range opts.Files {
		// Parse appID/filename
		appID, fileName := parseFileSpec(fileSpec)
		if appID == "" || fileName == "" {
			result.Errors = append(result.Errors, RestoreError{
				AppID:    appID,
				FileName: fileName,
				Error:    fmt.Errorf("invalid file specification: %s", fileSpec),
			})
			continue
		}

		// Source path in dotfiles
		sourcePath := b.GetMachineBackupPath(appID, opts.SourceMachine, fileName)

		// Check source exists
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			result.Errors = append(result.Errors, RestoreError{
				AppID:    appID,
				FileName: fileName,
				Error:    fmt.Errorf("source file not found: %w", err),
			})
			continue
		}

		// Get destination path (local config location)
		destPath := b.getLocalConfigPath(appID, fileName)
		if destPath == "" {
			result.Errors = append(result.Errors, RestoreError{
				AppID:    appID,
				FileName: fileName,
				Error:    fmt.Errorf("cannot determine local config path"),
			})
			continue
		}

		// Backup current file if requested and exists
		if opts.BackupCurrent {
			if _, err := os.Stat(destPath); err == nil {
				backupPath := b.getRestoreBackupPath(appID, fileName)
				if err := b.copyFile(destPath, backupPath); err != nil {
					result.Errors = append(result.Errors, RestoreError{
						AppID:    appID,
						FileName: fileName,
						Error:    fmt.Errorf("failed to backup current file: %w", err),
					})
					continue
				}

				result.BackedUpOld = append(result.BackedUpOld, BackedUpFile{
					AppID:    appID,
					FilePath: destPath,
					DestPath: backupPath,
				})
			}
		}

		// Copy from source machine to local
		if err := b.copyFile(sourcePath, destPath); err != nil {
			result.Errors = append(result.Errors, RestoreError{
				AppID:    appID,
				FileName: fileName,
				Error:    fmt.Errorf("failed to restore: %w", err),
			})
			continue
		}

		result.Restored = append(result.Restored, RestoredFile{
			AppID:      appID,
			FileName:   fileName,
			SourcePath: sourcePath,
			DestPath:   destPath,
			Size:       sourceInfo.Size(),
		})
	}

	return result, nil
}

// RestoreFile restores a single file from another machine
func (b *BackupManager) RestoreFile(sourceMachine, appID, fileName string, backupCurrent bool) error {
	opts := RestoreOptions{
		SourceMachine: sourceMachine,
		Files:         []string{appID + "/" + fileName},
		BackupCurrent: backupCurrent,
	}

	result, err := b.Restore(opts)
	if err != nil {
		return err
	}

	if len(result.Errors) > 0 {
		return result.Errors[0].Error
	}

	return nil
}

// GetRestorableFiles returns files that can be restored from a source machine
func (b *BackupManager) GetRestorableFiles(sourceMachine string) ([]RestorableFile, error) {
	var files []RestorableFile

	// Walk through dotfiles looking for source machine's files
	err := filepath.Walk(b.config.DotfilesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories
		if info.IsDir() {
			// Skip .git and .dotsync
			if info.Name() == ".git" || info.Name() == ".dotsync" {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path
		rel, err := filepath.Rel(b.config.DotfilesPath, path)
		if err != nil {
			return nil
		}

		// Parse path: appID/machineName/...relPath...
		parts := splitPath(rel)
		if len(parts) < 3 {
			return nil
		}

		appID, machineName := parts[0], parts[1]
		fileName := filepath.Join(parts[2:]...)

		if machineName != sourceMachine {
			return nil
		}

		files = append(files, RestorableFile{
			AppID:    appID,
			FileName: fileName,
			Path:     path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// RestorableFile represents a file that can be restored
type RestorableFile struct {
	AppID    string
	FileName string
	Path     string
	Size     int64
	ModTime  time.Time
}

// parseFileSpec parses "appID/fileName" format
func parseFileSpec(spec string) (appID, fileName string) {
	for i, c := range spec {
		if c == '/' {
			return spec[:i], spec[i+1:]
		}
	}
	return "", ""
}

// splitPath splits a path into components
func splitPath(path string) []string {
	var parts []string
	for path != "" {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == string(filepath.Separator) {
			break
		}
		path = filepath.Clean(dir)
	}
	return parts
}

// getLocalConfigPath returns the local config path for a file
// This would typically look up the app's config paths
func (b *BackupManager) getLocalConfigPath(appID, fileName string) string {
	homeDir, _ := os.UserHomeDir()

	// Common patterns for config locations
	// This is a simplified version - in reality, you'd look up the app definition
	commonPaths := []string{
		filepath.Join(homeDir, fileName),
		filepath.Join(homeDir, ".config", appID, fileName),
		filepath.Join(homeDir, "."+appID, fileName),
	}

	// Check if any existing path matches
	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default to home directory for dotfiles
	if len(fileName) > 0 && fileName[0] == '.' {
		return filepath.Join(homeDir, fileName)
	}

	return filepath.Join(homeDir, ".config", appID, fileName)
}

// getRestoreBackupPath returns the path to backup current file before restore
func (b *BackupManager) getRestoreBackupPath(appID, fileName string) string {
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(b.config.BackupPath, "restore", appID, fmt.Sprintf("%s.%s.bak", fileName, timestamp))
}

// CompareWithLocal compares a machine's backup with local files
func (b *BackupManager) CompareWithLocal(machineName string, appID, fileName string) (*FileComparison, error) {
	sourcePath := b.GetMachineBackupPath(appID, machineName, fileName)
	localPath := b.getLocalConfigPath(appID, fileName)

	comparison := &FileComparison{
		AppID:    appID,
		FileName: fileName,
	}

	// Check source
	if info, err := os.Stat(sourcePath); err == nil {
		comparison.SourceExists = true
		comparison.SourceSize = info.Size()
		comparison.SourceModTime = info.ModTime()
	}

	// Check local
	if info, err := os.Stat(localPath); err == nil {
		comparison.LocalExists = true
		comparison.LocalSize = info.Size()
		comparison.LocalModTime = info.ModTime()
	}

	return comparison, nil
}

// FileComparison contains comparison between source and local file
type FileComparison struct {
	AppID         string
	FileName      string
	SourceExists  bool
	SourceSize    int64
	SourceModTime time.Time
	LocalExists   bool
	LocalSize     int64
	LocalModTime  time.Time
}

// IsDifferent returns true if files are different
func (fc *FileComparison) IsDifferent() bool {
	if fc.SourceExists != fc.LocalExists {
		return true
	}
	if !fc.SourceExists {
		return false
	}
	return fc.SourceSize != fc.LocalSize || !fc.SourceModTime.Equal(fc.LocalModTime)
}

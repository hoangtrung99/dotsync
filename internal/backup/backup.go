package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"dotsync/internal/config"
	"dotsync/internal/modes"
	"dotsync/internal/models"
)

// BackupManager handles backup operations for machine-specific files
type BackupManager struct {
	config      *config.Config
	modesConfig *modes.ModesConfig
}

// BackupResult contains the result of a backup operation
type BackupResult struct {
	BackedUp []BackedUpFile
	Skipped  []SkippedFile
	Errors   []BackupError
}

// BackedUpFile represents a successfully backed up file
type BackedUpFile struct {
	AppID    string
	FilePath string
	DestPath string
	Size     int64
}

// SkippedFile represents a file that was skipped (sync mode)
type SkippedFile struct {
	AppID    string
	FilePath string
	Reason   string
}

// BackupError represents an error during backup
type BackupError struct {
	AppID    string
	FilePath string
	Error    error
}

// Machine represents a machine with backup data
type Machine struct {
	Name     string    `json:"name"`
	LastSync time.Time `json:"last_sync"`
}

// MachinesFile is the structure for machines.json
type MachinesFile struct {
	Machines []Machine `json:"machines"`
}

// New creates a new BackupManager
func New(cfg *config.Config, modesCfg *modes.ModesConfig) *BackupManager {
	return &BackupManager{
		config:      cfg,
		modesConfig: modesCfg,
	}
}

// Backup backs up files from apps that are in backup mode
func (b *BackupManager) Backup(apps []*models.App) (*BackupResult, error) {
	result := &BackupResult{
		BackedUp: []BackedUpFile{},
		Skipped:  []SkippedFile{},
		Errors:   []BackupError{},
	}

	for _, app := range apps {
		if !app.Selected {
			continue
		}

		for _, file := range app.Files {
			if !file.Selected {
				continue
			}

			mode := b.modesConfig.GetMode(app.ID, file.Path)

			if mode == modes.ModeSync {
				result.Skipped = append(result.Skipped, SkippedFile{
					AppID:    app.ID,
					FilePath: file.Path,
					Reason:   "sync mode",
				})
				continue
			}

			// Backup mode - copy to machine folder
			destPath := b.getBackupDestPath(app.ID, file.Name)
			if err := b.copyFile(file.Path, destPath); err != nil {
				result.Errors = append(result.Errors, BackupError{
					AppID:    app.ID,
					FilePath: file.Path,
					Error:    err,
				})
				continue
			}

			result.BackedUp = append(result.BackedUp, BackedUpFile{
				AppID:    app.ID,
				FilePath: file.Path,
				DestPath: destPath,
				Size:     file.Size,
			})
		}
	}

	// Update machines.json
	if len(result.BackedUp) > 0 {
		if err := b.updateMachinesFile(); err != nil {
			return result, fmt.Errorf("backup succeeded but failed to update machines.json: %w", err)
		}
	}

	return result, nil
}

// BackupFile backs up a single file
func (b *BackupManager) BackupFile(appID string, file models.File) error {
	mode := b.modesConfig.GetMode(appID, file.Path)
	if mode != modes.ModeBackup {
		return fmt.Errorf("file is in sync mode, not backup mode")
	}

	destPath := b.getBackupDestPath(appID, file.Name)
	if err := b.copyFile(file.Path, destPath); err != nil {
		return err
	}

	return b.updateMachinesFile()
}

// ListMachines returns all machines with backup data
func (b *BackupManager) ListMachines() ([]Machine, error) {
	machinesFile, err := b.loadMachinesFile()
	if err != nil {
		if os.IsNotExist(err) {
			return []Machine{}, nil
		}
		return nil, err
	}

	return machinesFile.Machines, nil
}

// ListMachineFiles returns all backed up files for a specific machine
func (b *BackupManager) ListMachineFiles(machineName string) ([]string, error) {
	var files []string

	// Walk through dotfiles directory looking for machine folders
	err := filepath.Walk(b.config.DotfilesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip .git and .dotsync directories
		if info.IsDir() && (info.Name() == ".git" || info.Name() == ".dotsync") {
			return filepath.SkipDir
		}

		// Check if this is a file inside a machine folder
		rel, _ := filepath.Rel(b.config.DotfilesPath, path)
		parts := filepath.SplitList(rel)
		if len(parts) >= 2 {
			// Check if parent directory is the machine name
			dir := filepath.Dir(rel)
			if filepath.Base(dir) == machineName && !info.IsDir() {
				files = append(files, rel)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// GetMachineBackupPath returns the backup path for a machine
func (b *BackupManager) GetMachineBackupPath(appID, machineName, fileName string) string {
	return filepath.Join(b.config.DotfilesPath, appID, machineName, fileName)
}

// getBackupDestPath returns the destination path for a backup file
func (b *BackupManager) getBackupDestPath(appID, fileName string) string {
	return filepath.Join(b.config.DotfilesPath, appID, b.modesConfig.MachineName, fileName)
}

// copyFile copies a file from src to dst, creating directories as needed
func (b *BackupManager) copyFile(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

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

// machinesFilePath returns the path to machines.json
func (b *BackupManager) machinesFilePath() string {
	return filepath.Join(b.config.DotfilesPath, ".dotsync", "machines.json")
}

// loadMachinesFile loads the machines.json file
func (b *BackupManager) loadMachinesFile() (*MachinesFile, error) {
	data, err := os.ReadFile(b.machinesFilePath())
	if err != nil {
		return nil, err
	}

	var mf MachinesFile
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, err
	}

	return &mf, nil
}

// saveMachinesFile saves the machines.json file
func (b *BackupManager) saveMachinesFile(mf *MachinesFile) error {
	// Create .dotsync directory
	dotsyncDir := filepath.Join(b.config.DotfilesPath, ".dotsync")
	if err := os.MkdirAll(dotsyncDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(b.machinesFilePath(), data, 0644)
}

// updateMachinesFile updates the machines.json with current machine
func (b *BackupManager) updateMachinesFile() error {
	mf, err := b.loadMachinesFile()
	if err != nil {
		if os.IsNotExist(err) {
			mf = &MachinesFile{Machines: []Machine{}}
		} else {
			return err
		}
	}

	// Find and update or add current machine
	found := false
	for i, m := range mf.Machines {
		if m.Name == b.modesConfig.MachineName {
			mf.Machines[i].LastSync = time.Now()
			found = true
			break
		}
	}

	if !found {
		mf.Machines = append(mf.Machines, Machine{
			Name:     b.modesConfig.MachineName,
			LastSync: time.Now(),
		})
	}

	return b.saveMachinesFile(mf)
}

// HasBackups checks if there are any backups for the current machine
func (b *BackupManager) HasBackups() bool {
	machines, err := b.ListMachines()
	if err != nil {
		return false
	}

	for _, m := range machines {
		if m.Name == b.modesConfig.MachineName {
			return true
		}
	}

	return false
}

// GetBackupStats returns statistics about backups
func (b *BackupManager) GetBackupStats() (machineCount int, lastSync time.Time, err error) {
	machines, err := b.ListMachines()
	if err != nil {
		return 0, time.Time{}, err
	}

	machineCount = len(machines)

	for _, m := range machines {
		if m.Name == b.modesConfig.MachineName {
			lastSync = m.LastSync
			break
		}
	}

	return machineCount, lastSync, nil
}

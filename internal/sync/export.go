package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"dotsync/internal/config"
	"dotsync/internal/models"
)

// Exporter handles exporting configs from system to dotfiles
type Exporter struct {
	config *config.Config
}

// NewExporter creates a new Exporter
func NewExporter(cfg *config.Config) *Exporter {
	return &Exporter{config: cfg}
}

// ExportResult holds the result of an export operation
type ExportResult struct {
	App       *models.App
	File      models.File
	Success   bool
	Error     error
	Encrypted bool
}

// ExportApp exports all selected files from an app
func (e *Exporter) ExportApp(app *models.App) ([]ExportResult, error) {
	var results []ExportResult

	destDir := e.config.GetDestPath(app.ID)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, file := range app.Files {
		if !file.Selected {
			continue
		}

		result := ExportResult{
			App:       app,
			File:      file,
			Encrypted: file.Encrypted,
		}

		destPath := filepath.Join(destDir, file.RelPath)

		if file.IsDir {
			err := e.copyDir(file.Path, destPath)
			result.Success = err == nil
			result.Error = err
		} else {
			err := e.copyFile(file.Path, destPath)
			result.Success = err == nil
			result.Error = err
		}

		results = append(results, result)
	}

	return results, nil
}

// ExportAll exports all selected apps and files
func (e *Exporter) ExportAll(apps []*models.App) ([]ExportResult, error) {
	var allResults []ExportResult

	for _, app := range apps {
		if !app.Selected {
			continue
		}

		results, err := e.ExportApp(app)
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// copyFile copies a single file
func (e *Exporter) copyFile(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// copyDir copies a directory recursively
func (e *Exporter) copyDir(src, dst string) error {
	// Get source info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Skip hidden files and common unwanted files
		if shouldSkipFile(entry.Name()) {
			continue
		}

		if entry.IsDir() {
			if err := e.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := e.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// shouldSkipFile returns true if the file should be skipped
func shouldSkipFile(name string) bool {
	skipPatterns := []string{
		".DS_Store",
		".git",
		"node_modules",
		"__pycache__",
		".cache",
		"Cache",
	}

	for _, pattern := range skipPatterns {
		if name == pattern {
			return true
		}
	}
	return false
}

// Backup backs up a file/directory before importing
func Backup(path string, backupDir string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil // Nothing to backup
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, timestamp, filepath.Base(path))

	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	exporter := &Exporter{}
	if info.IsDir() {
		err = exporter.copyDir(path, backupPath)
	} else {
		err = exporter.copyFile(path, backupPath)
	}

	return backupPath, err
}

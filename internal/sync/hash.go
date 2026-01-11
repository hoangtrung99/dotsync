package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// ComputeFileHash computes SHA256 hash of a file
func ComputeFileHash(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return ComputeDirHash(path)
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeDirHash computes a combined hash of all files in a directory
func ComputeDirHash(dirPath string) (string, error) {
	hasher := sha256.New()

	var filePaths []string
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !d.IsDir() && !shouldSkipFile(d.Name()) {
			relPath, _ := filepath.Rel(dirPath, path)
			filePaths = append(filePaths, relPath)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// Sort for consistent ordering
	sort.Strings(filePaths)

	for _, relPath := range filePaths {
		fullPath := filepath.Join(dirPath, relPath)

		// Hash the relative path
		hasher.Write([]byte(relPath))

		// Hash the file content
		file, err := os.Open(fullPath)
		if err != nil {
			continue
		}
		io.Copy(hasher, file)
		file.Close()
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// QuickHash returns first 8 chars of hash for display
func QuickHash(hash string) string {
	if len(hash) >= 8 {
		return hash[:8]
	}
	return hash
}

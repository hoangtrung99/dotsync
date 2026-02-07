package modes

import (
	"path/filepath"
	"strings"
)

// normalizeFilePath creates a consistent file key for the files map
// Format: appID/filename (e.g., "zsh/.zshrc")
func normalizeFilePath(appID, filePath string) string {
	// If filePath already contains the appID prefix, return as-is
	if strings.HasPrefix(filePath, appID+"/") {
		return filePath
	}

	// Get just the filename from the path
	filename := filepath.Base(filePath)

	// Combine with appID
	return appID + "/" + filename
}

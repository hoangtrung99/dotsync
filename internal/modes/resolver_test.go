package modes

import (
	"testing"
)

func TestNormalizeFilePath(t *testing.T) {
	tests := []struct {
		appID    string
		filePath string
		expected string
	}{
		{"zsh", ".zshrc", "zsh/.zshrc"},
		{"zsh", "zsh/.zshrc", "zsh/.zshrc"},
		{"git", "/home/user/.gitconfig", "git/.gitconfig"},
	}

	for _, tt := range tests {
		result := normalizeFilePath(tt.appID, tt.filePath)
		if result != tt.expected {
			t.Errorf("normalizeFilePath(%s, %s) = %s, want %s", tt.appID, tt.filePath, result, tt.expected)
		}
	}
}

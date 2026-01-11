package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MergeResolution represents how a conflict hunk should be resolved
type MergeResolution int

const (
	ResolutionPending MergeResolution = iota
	ResolutionKeepLocal
	ResolutionUseDotfiles
	ResolutionManual
)

// MergeHunk represents a single mergeable unit of conflict
type MergeHunk struct {
	Index           int             // Hunk index
	LocalLines      []string        // Lines from local version
	DotfilesLines   []string        // Lines from dotfiles version
	ContextBefore   []string        // Context lines before conflict
	ContextAfter    []string        // Context lines after conflict
	StartLine       int             // Starting line number in the file
	Resolution      MergeResolution // How this hunk was resolved
	ResolvedContent []string        // Content after resolution
}

// MergeResult represents the result of a merge operation
type MergeResult struct {
	FilePath        string
	LocalPath       string
	DotfilesPath    string
	Hunks           []MergeHunk
	ResolvedHunks   int
	TotalHunks      int
	IsFullyResolved bool
	MergedContent   string
}

// NewMergeResult creates a MergeResult from a DiffResult
func NewMergeResult(diffResult *DiffResult, localPath, dotfilesPath string) *MergeResult {
	result := &MergeResult{
		FilePath:     diffResult.OldPath,
		LocalPath:    localPath,
		DotfilesPath: dotfilesPath,
		TotalHunks:   len(diffResult.Hunks),
	}

	// Convert diff hunks to merge hunks
	for i, hunk := range diffResult.Hunks {
		mergeHunk := MergeHunk{
			Index:      i,
			StartLine:  hunk.StartOld,
			Resolution: ResolutionPending,
		}

		// Separate lines by type
		for _, line := range hunk.DiffLines {
			switch line.Type {
			case DiffDelete:
				// Deleted lines are from local (old)
				mergeHunk.LocalLines = append(mergeHunk.LocalLines, line.Content)
			case DiffInsert:
				// Inserted lines are from dotfiles (new)
				mergeHunk.DotfilesLines = append(mergeHunk.DotfilesLines, line.Content)
			case DiffEqual:
				// Context lines - add to appropriate place
				if len(mergeHunk.LocalLines) == 0 && len(mergeHunk.DotfilesLines) == 0 {
					mergeHunk.ContextBefore = append(mergeHunk.ContextBefore, line.Content)
				} else {
					mergeHunk.ContextAfter = append(mergeHunk.ContextAfter, line.Content)
				}
			}
		}

		result.Hunks = append(result.Hunks, mergeHunk)
	}

	return result
}

// ResolveHunk resolves a specific hunk with the given resolution
func (m *MergeResult) ResolveHunk(hunkIndex int, resolution MergeResolution) {
	if hunkIndex < 0 || hunkIndex >= len(m.Hunks) {
		return
	}

	hunk := &m.Hunks[hunkIndex]
	hunk.Resolution = resolution

	switch resolution {
	case ResolutionKeepLocal:
		hunk.ResolvedContent = hunk.LocalLines
	case ResolutionUseDotfiles:
		hunk.ResolvedContent = hunk.DotfilesLines
	}

	m.updateResolvedCount()
}

// ResolveHunkManual resolves a hunk with custom content
func (m *MergeResult) ResolveHunkManual(hunkIndex int, content []string) {
	if hunkIndex < 0 || hunkIndex >= len(m.Hunks) {
		return
	}

	hunk := &m.Hunks[hunkIndex]
	hunk.Resolution = ResolutionManual
	hunk.ResolvedContent = content

	m.updateResolvedCount()
}

// updateResolvedCount updates the resolved hunk counter
func (m *MergeResult) updateResolvedCount() {
	m.ResolvedHunks = 0
	for _, hunk := range m.Hunks {
		if hunk.Resolution != ResolutionPending {
			m.ResolvedHunks++
		}
	}
	m.IsFullyResolved = m.ResolvedHunks == m.TotalHunks
}

// KeepAllLocal resolves all hunks by keeping local version
func (m *MergeResult) KeepAllLocal() {
	for i := range m.Hunks {
		m.ResolveHunk(i, ResolutionKeepLocal)
	}
}

// UseAllDotfiles resolves all hunks by using dotfiles version
func (m *MergeResult) UseAllDotfiles() {
	for i := range m.Hunks {
		m.ResolveHunk(i, ResolutionUseDotfiles)
	}
}

// GenerateMergedContent generates the final merged content
func (m *MergeResult) GenerateMergedContent() (string, error) {
	if !m.IsFullyResolved {
		return "", fmt.Errorf("not all hunks are resolved (%d/%d)", m.ResolvedHunks, m.TotalHunks)
	}

	// Read the base file (local version)
	content, err := os.ReadFile(m.LocalPath)
	if err != nil {
		return "", fmt.Errorf("cannot read local file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var result []string

	// Simple approach: rebuild the file with resolved hunks
	// For a more sophisticated merge, we'd need line-by-line tracking
	lineIndex := 0
	for _, hunk := range m.Hunks {
		// Add lines before this hunk
		for lineIndex < hunk.StartLine-1 && lineIndex < len(lines) {
			result = append(result, lines[lineIndex])
			lineIndex++
		}

		// Add context before
		result = append(result, hunk.ContextBefore...)

		// Add resolved content
		result = append(result, hunk.ResolvedContent...)

		// Skip the original local lines that were part of this hunk
		lineIndex += len(hunk.LocalLines)

		// Add context after
		result = append(result, hunk.ContextAfter...)
	}

	// Add remaining lines
	for lineIndex < len(lines) {
		result = append(result, lines[lineIndex])
		lineIndex++
	}

	m.MergedContent = strings.Join(result, "\n")
	return m.MergedContent, nil
}

// WriteMergedFile writes the merged content to the local path
func (m *MergeResult) WriteMergedFile() error {
	if m.MergedContent == "" {
		if _, err := m.GenerateMergedContent(); err != nil {
			return err
		}
	}

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(m.LocalPath), 0755); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}

	return os.WriteFile(m.LocalPath, []byte(m.MergedContent), 0644)
}

// FormatHunkPreview formats a hunk for display in the UI
func (h *MergeHunk) FormatHunkPreview(maxLines int) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("=== Hunk %d ===\n", h.Index+1))

	// Show local version
	b.WriteString("<<<<<<< LOCAL\n")
	for i, line := range h.LocalLines {
		if maxLines > 0 && i >= maxLines {
			b.WriteString(fmt.Sprintf("... and %d more lines\n", len(h.LocalLines)-maxLines))
			break
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("=======\n")

	// Show dotfiles version
	for i, line := range h.DotfilesLines {
		if maxLines > 0 && i >= maxLines {
			b.WriteString(fmt.Sprintf("... and %d more lines\n", len(h.DotfilesLines)-maxLines))
			break
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(">>>>>>> DOTFILES\n")

	return b.String()
}

// ResolutionString returns a string representation of the resolution
func (r MergeResolution) String() string {
	switch r {
	case ResolutionPending:
		return "Pending"
	case ResolutionKeepLocal:
		return "Keep Local"
	case ResolutionUseDotfiles:
		return "Use Dotfiles"
	case ResolutionManual:
		return "Manual"
	default:
		return "Unknown"
	}
}

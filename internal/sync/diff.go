package sync

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffType represents the type of diff operation
type DiffType int

const (
	DiffEqual DiffType = iota
	DiffInsert
	DiffDelete
)

// DiffLine represents a single line in the diff
type DiffLine struct {
	Type    DiffType
	Content string
	LineNum int // Line number in original (for delete) or new (for insert)
}

// DiffHunk represents a group of changes
type DiffHunk struct {
	StartOld  int        // Starting line in old file
	StartNew  int        // Starting line in new file
	LinesOld  []string   // Lines from old file
	LinesNew  []string   // Lines from new file
	DiffLines []DiffLine // Unified diff lines
}

// DiffResult contains the complete diff between two files
type DiffResult struct {
	OldPath      string
	NewPath      string
	OldExists    bool
	NewExists    bool
	Identical    bool
	Hunks        []DiffHunk
	LinesAdded   int
	LinesRemoved int
}

// ComputeDiff computes the diff between two files using go-diff library
func ComputeDiff(oldPath, newPath string) (*DiffResult, error) {
	result := &DiffResult{
		OldPath: oldPath,
		NewPath: newPath,
	}

	// Read old file
	oldContent, oldErr := os.ReadFile(oldPath)
	result.OldExists = oldErr == nil
	oldText := ""
	if oldErr == nil {
		oldText = string(oldContent)
	}

	// Read new file
	newContent, newErr := os.ReadFile(newPath)
	result.NewExists = newErr == nil
	newText := ""
	if newErr == nil {
		newText = string(newContent)
	}

	// Handle cases where one or both don't exist
	if !result.OldExists && !result.NewExists {
		result.Identical = true
		return result, nil
	}

	if !result.OldExists {
		// All lines are additions
		newLines := strings.Split(newText, "\n")
		result.Hunks = []DiffHunk{{
			StartNew:  1,
			LinesNew:  newLines,
			DiffLines: linesToDiff(newLines, DiffInsert),
		}}
		result.LinesAdded = len(newLines)
		return result, nil
	}

	if !result.NewExists {
		// All lines are deletions
		oldLines := strings.Split(oldText, "\n")
		result.Hunks = []DiffHunk{{
			StartOld:  1,
			LinesOld:  oldLines,
			DiffLines: linesToDiff(oldLines, DiffDelete),
		}}
		result.LinesRemoved = len(oldLines)
		return result, nil
	}

	// Use go-diff library for line-by-line diff
	dmp := diffmatchpatch.New()

	// Convert to rune-based for better handling
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	// Use line mode diff for better results on text files
	chars1, chars2, lineArray := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffMain(chars1, chars2, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)
	diffs = dmp.DiffCleanupSemantic(diffs)

	// Check if identical
	if len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual {
		result.Identical = true
		return result, nil
	}

	// Convert go-diff output to our format
	hunks := convertToHunks(diffs, oldLines, newLines)
	result.Hunks = hunks

	for _, hunk := range hunks {
		result.LinesAdded += len(hunk.LinesNew)
		result.LinesRemoved += len(hunk.LinesOld)
	}

	result.Identical = len(hunks) == 0

	return result, nil
}

// convertToHunks converts go-diff output to our DiffHunk format
func convertToHunks(diffs []diffmatchpatch.Diff, oldLines, newLines []string) []DiffHunk {
	var hunks []DiffHunk
	var currentHunk *DiffHunk
	oldLineNum := 1
	newLineNum := 1
	contextLines := 3
	equalsSinceChange := 0

	for _, d := range diffs {
		lines := strings.Split(strings.TrimSuffix(d.Text, "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" && d.Text == "" {
			continue
		}

		switch d.Type {
		case diffmatchpatch.DiffEqual:
			equalsSinceChange += len(lines)

			if currentHunk != nil {
				// Add context lines after changes
				for i, line := range lines {
					if i >= contextLines {
						// End current hunk
						hunks = append(hunks, *currentHunk)
						currentHunk = nil
						break
					}
					currentHunk.DiffLines = append(currentHunk.DiffLines, DiffLine{
						Type:    DiffEqual,
						Content: line,
						LineNum: oldLineNum + i,
					})
				}
			}

			oldLineNum += len(lines)
			newLineNum += len(lines)

		case diffmatchpatch.DiffDelete:
			if currentHunk == nil {
				currentHunk = &DiffHunk{
					StartOld: max(1, oldLineNum-contextLines),
					StartNew: max(1, newLineNum-contextLines),
				}
				// Add context before
				if equalsSinceChange > 0 {
					contextStart := max(0, len(oldLines)-equalsSinceChange)
					for i := max(0, contextStart); i < contextStart+min(contextLines, equalsSinceChange) && i < len(oldLines); i++ {
						if i < oldLineNum-1 {
							currentHunk.DiffLines = append(currentHunk.DiffLines, DiffLine{
								Type:    DiffEqual,
								Content: oldLines[i],
								LineNum: i + 1,
							})
						}
					}
				}
			}
			equalsSinceChange = 0

			for i, line := range lines {
				currentHunk.DiffLines = append(currentHunk.DiffLines, DiffLine{
					Type:    DiffDelete,
					Content: line,
					LineNum: oldLineNum + i,
				})
				currentHunk.LinesOld = append(currentHunk.LinesOld, line)
			}

			oldLineNum += len(lines)

		case diffmatchpatch.DiffInsert:
			if currentHunk == nil {
				currentHunk = &DiffHunk{
					StartOld: max(1, oldLineNum-contextLines),
					StartNew: max(1, newLineNum-contextLines),
				}
			}
			equalsSinceChange = 0

			for i, line := range lines {
				currentHunk.DiffLines = append(currentHunk.DiffLines, DiffLine{
					Type:    DiffInsert,
					Content: line,
					LineNum: newLineNum + i,
				})
				currentHunk.LinesNew = append(currentHunk.LinesNew, line)
			}

			newLineNum += len(lines)
		}
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks
}

// readLines reads a file into lines (kept for compatibility)
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// linesToDiff converts lines to DiffLines with given type
func linesToDiff(lines []string, diffType DiffType) []DiffLine {
	result := make([]DiffLine, len(lines))
	for i, line := range lines {
		result[i] = DiffLine{
			Type:    diffType,
			Content: line,
			LineNum: i + 1,
		}
	}
	return result
}

// FormatUnifiedDiff formats the diff result as unified diff
func FormatUnifiedDiff(result *DiffResult) string {
	var sb strings.Builder

	sb.WriteString("--- " + result.OldPath + "\n")
	sb.WriteString("+++ " + result.NewPath + "\n")

	for _, hunk := range result.Hunks {
		for _, line := range hunk.DiffLines {
			switch line.Type {
			case DiffEqual:
				sb.WriteString(" " + line.Content + "\n")
			case DiffInsert:
				sb.WriteString("+" + line.Content + "\n")
			case DiffDelete:
				sb.WriteString("-" + line.Content + "\n")
			}
		}
	}

	return sb.String()
}

// HasChanges returns true if there are any changes
func (d *DiffResult) HasChanges() bool {
	return !d.Identical
}

// Summary returns a brief summary of changes
func (d *DiffResult) Summary() string {
	if d.Identical {
		return "No changes"
	}

	var parts []string
	if d.LinesAdded > 0 {
		parts = append(parts, "+"+strconv.Itoa(d.LinesAdded))
	}
	if d.LinesRemoved > 0 {
		parts = append(parts, "-"+strconv.Itoa(d.LinesRemoved))
	}
	return strings.Join(parts, " ")
}

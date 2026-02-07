package editor

import (
	"os/exec"
)

// Cursor implements Editor interface for Cursor IDE
type Cursor struct {
	baseEditor
}

// NewCursor creates a new Cursor editor instance
func NewCursor() Editor {
	return &Cursor{
		baseEditor: baseEditor{
			name:    "Cursor",
			command: "cursor",
		},
	}
}

// OpenMerge opens Cursor's 3-way merge editor
// Command: cursor --wait --merge LOCAL REMOTE BASE MERGED
func (e *Cursor) OpenMerge(local, remote, merged string) error {
	// Cursor uses the same merge command as VS Code
	e.cmd = exec.Command(e.command, "--wait", "--merge", local, remote, merged, merged)
	return e.cmd.Start()
}

// OpenDiff opens Cursor's diff view between two files
// Command: cursor --wait --diff FILE1 FILE2
func (e *Cursor) OpenDiff(file1, file2 string) error {
	e.cmd = exec.Command(e.command, "--wait", "--diff", file1, file2)
	return e.cmd.Start()
}

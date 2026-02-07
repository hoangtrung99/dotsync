package editor

import (
	"os/exec"
)

// Zed implements Editor interface for Zed editor
type Zed struct {
	baseEditor
}

// NewZed creates a new Zed editor instance
func NewZed() Editor {
	return &Zed{
		baseEditor: baseEditor{
			name:    "Zed",
			command: "zed",
		},
	}
}

// OpenMerge opens files in Zed for manual merge
// Command: zed --wait LOCAL REMOTE MERGED
// Note: Zed doesn't have a dedicated merge mode, so we open all three files
func (e *Zed) OpenMerge(local, remote, merged string) error {
	e.cmd = exec.Command(e.command, "--wait", local, remote, merged)
	return e.cmd.Start()
}

// OpenDiff opens Zed's diff view between two files
// Command: zed --wait FILE1 FILE2
func (e *Zed) OpenDiff(file1, file2 string) error {
	e.cmd = exec.Command(e.command, "--wait", file1, file2)
	return e.cmd.Start()
}

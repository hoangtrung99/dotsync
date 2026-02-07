package editor

import (
	"os/exec"
)

// VSCode implements Editor interface for Visual Studio Code
type VSCode struct {
	baseEditor
}

// NewVSCode creates a new VS Code editor instance
func NewVSCode() Editor {
	return &VSCode{
		baseEditor: baseEditor{
			name:    "VS Code",
			command: "code",
		},
	}
}

// OpenMerge opens VS Code's 3-way merge editor
// Command: code --wait --merge LOCAL REMOTE BASE MERGED
func (e *VSCode) OpenMerge(local, remote, merged string) error {
	// VS Code merge command expects: LOCAL REMOTE BASE MERGED
	// For our use case, we use merged as both base and output
	e.cmd = exec.Command(e.command, "--wait", "--merge", local, remote, merged, merged)
	return e.cmd.Start()
}

// OpenDiff opens VS Code's diff view between two files
// Command: code --wait --diff FILE1 FILE2
func (e *VSCode) OpenDiff(file1, file2 string) error {
	e.cmd = exec.Command(e.command, "--wait", "--diff", file1, file2)
	return e.cmd.Start()
}

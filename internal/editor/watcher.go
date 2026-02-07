package editor

import (
	"context"
	"os"
	"time"
)

// FileWatcher watches for file changes to detect when merge is complete
type FileWatcher struct {
	path     string
	interval time.Duration
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(path string) *FileWatcher {
	return &FileWatcher{
		path:     path,
		interval: 500 * time.Millisecond,
	}
}

// WatchResult contains the result of watching a file
type WatchResult struct {
	Modified bool
	Error    error
}

// WaitForChange blocks until the file is modified or context is cancelled
// Returns true if file was modified, false if cancelled or error
func (w *FileWatcher) WaitForChange(ctx context.Context) WatchResult {
	initialInfo, err := os.Stat(w.path)
	if err != nil {
		return WatchResult{Error: err}
	}
	initialModTime := initialInfo.ModTime()
	initialSize := initialInfo.Size()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return WatchResult{Modified: false, Error: ctx.Err()}
		case <-ticker.C:
			info, err := os.Stat(w.path)
			if err != nil {
				continue // File might be temporarily unavailable during save
			}

			// Check if file was modified
			if info.ModTime() != initialModTime || info.Size() != initialSize {
				return WatchResult{Modified: true}
			}
		}
	}
}

// WaitForSave waits for the file to be saved (modification time changes)
// with a timeout
func (w *FileWatcher) WaitForSave(timeout time.Duration) WatchResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return w.WaitForChange(ctx)
}

// MergeWatcher watches for merge completion in an editor
type MergeWatcher struct {
	editor   Editor
	merged   string
	watcher  *FileWatcher
}

// NewMergeWatcher creates a watcher for merge operations
func NewMergeWatcher(editor Editor, mergedPath string) *MergeWatcher {
	return &MergeWatcher{
		editor:  editor,
		merged:  mergedPath,
		watcher: NewFileWatcher(mergedPath),
	}
}

// Wait blocks until the merge is complete (editor closes or file is saved)
func (m *MergeWatcher) Wait(ctx context.Context) WatchResult {
	// Create a channel to signal when editor closes
	editorDone := make(chan error, 1)
	go func() {
		editorDone <- m.editor.Wait()
	}()

	// Watch for file changes
	fileChanged := make(chan WatchResult, 1)
	go func() {
		fileChanged <- m.watcher.WaitForChange(ctx)
	}()

	select {
	case <-ctx.Done():
		return WatchResult{Modified: false, Error: ctx.Err()}
	case err := <-editorDone:
		if err != nil {
			return WatchResult{Error: err}
		}
		// Editor closed, check if file was modified
		return m.checkFileModified()
	case result := <-fileChanged:
		return result
	}
}

// checkFileModified checks if merged file exists and has content
func (m *MergeWatcher) checkFileModified() WatchResult {
	info, err := os.Stat(m.merged)
	if err != nil {
		return WatchResult{Error: err}
	}
	return WatchResult{Modified: info.Size() > 0}
}

package quicksync

import (
	"os"
	"path/filepath"
	"testing"

	"dotsync/internal/config"
	"dotsync/internal/modes"
	"dotsync/internal/models"
)

func TestFileStateString(t *testing.T) {
	tests := []struct {
		state    FileState
		expected string
	}{
		{StateSynced, "synced"},
		{StateLocalModified, "local modified"},
		{StateRemoteModified, "remote modified"},
		{StateConflict, "conflict"},
		{StateLocalNew, "local new"},
		{StateRemoteNew, "remote new"},
		{StateDeleted, "deleted"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("FileState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestFileStateIcon(t *testing.T) {
	tests := []struct {
		state    FileState
		expected string
	}{
		{StateSynced, "✓"},
		{StateLocalModified, "↑"},
		{StateRemoteModified, "↓"},
		{StateConflict, "⚡"},
		{StateLocalNew, "+"},
		{StateRemoteNew, "↓"},
		{StateDeleted, "✗"},
	}

	for _, tt := range tests {
		if got := tt.state.Icon(); got != tt.expected {
			t.Errorf("FileState(%d).Icon() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestDetectionResultHasChanges(t *testing.T) {
	tests := []struct {
		name     string
		result   DetectionResult
		expected bool
	}{
		{
			name:     "no changes",
			result:   DetectionResult{},
			expected: false,
		},
		{
			name:     "local modified",
			result:   DetectionResult{LocalModified: 1},
			expected: true,
		},
		{
			name:     "remote updated",
			result:   DetectionResult{RemoteUpdated: 2},
			expected: true,
		},
		{
			name:     "conflicts",
			result:   DetectionResult{Conflicts: 3},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasChanges(); got != tt.expected {
				t.Errorf("HasChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetectionResultIsAllSynced(t *testing.T) {
	allSynced := DetectionResult{}
	if !allSynced.IsAllSynced() {
		t.Error("expected IsAllSynced() = true for empty result")
	}

	notSynced := DetectionResult{LocalModified: 1}
	if notSynced.IsAllSynced() {
		t.Error("expected IsAllSynced() = false when LocalModified > 0")
	}
}

func TestResolveActionString(t *testing.T) {
	tests := []struct {
		action   ResolveAction
		expected string
	}{
		{ActionNone, "none"},
		{ActionPush, "push"},
		{ActionPull, "pull"},
		{ActionMerge, "merge"},
		{ActionSkip, "skip"},
	}

	for _, tt := range tests {
		if got := tt.action.String(); got != tt.expected {
			t.Errorf("ResolveAction(%d).String() = %q, want %q", tt.action, got, tt.expected)
		}
	}
}

func TestActionTypeString(t *testing.T) {
	tests := []struct {
		action   ActionType
		expected string
	}{
		{ActionSynced, "synced"},
		{ActionBackedUp, "backed up"},
		{ActionPulled, "pulled"},
		{ActionMerged, "merged"},
		{ActionPending, "pending"},
		{ActionFailed, "failed"},
	}

	for _, tt := range tests {
		if got := tt.action.String(); got != tt.expected {
			t.Errorf("ActionType(%d).String() = %q, want %q", tt.action, got, tt.expected)
		}
	}
}

func TestGenerateCommitMessage(t *testing.T) {
	tests := []struct {
		name     string
		files    []FileInfo
		contains string
	}{
		{
			name:     "empty",
			files:    []FileInfo{},
			contains: "update configs",
		},
		{
			name:     "single app",
			files:    []FileInfo{{AppID: "zsh"}},
			contains: "zsh",
		},
		{
			name: "multiple apps",
			files: []FileInfo{
				{AppID: "zsh"},
				{AppID: "git"},
			},
			contains: "zsh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GenerateCommitMessage(tt.files)
			if !contains(msg, tt.contains) {
				t.Errorf("GenerateCommitMessage() = %q, want to contain %q", msg, tt.contains)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestQuickSyncNew(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	dotfilesPath := filepath.Join(tmpDir, "dotfiles")
	os.MkdirAll(dotfilesPath, 0755)

	cfg := &config.Config{
		DotfilesPath: dotfilesPath,
		BackupPath:   filepath.Join(tmpDir, "backup"),
	}

	modesCfg := modes.Default()

	qs := New(cfg, modesCfg)

	if qs == nil {
		t.Fatal("New() returned nil")
	}

	if qs.config != cfg {
		t.Error("config not set correctly")
	}

	if qs.modesConfig != modesCfg {
		t.Error("modesConfig not set correctly")
	}

	if qs.detector == nil {
		t.Error("detector is nil")
	}

	if qs.resolver == nil {
		t.Error("resolver is nil")
	}
}

func TestQuickSyncRunEmptyApps(t *testing.T) {
	tmpDir := t.TempDir()
	dotfilesPath := filepath.Join(tmpDir, "dotfiles")
	os.MkdirAll(dotfilesPath, 0755)

	cfg := &config.Config{
		DotfilesPath: dotfilesPath,
		BackupPath:   filepath.Join(tmpDir, "backup"),
	}

	modesCfg := modes.Default()
	qs := New(cfg, modesCfg)

	result := qs.Run([]*models.App{})

	if result.Action != ActionSynced {
		t.Errorf("expected ActionSynced for empty apps, got %v", result.Action)
	}

	if result.BackedUpCount != 0 {
		t.Errorf("expected BackedUpCount = 0, got %d", result.BackedUpCount)
	}
}

func TestResultHasSyncPending(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected bool
	}{
		{
			name:     "no pending",
			result:   Result{},
			expected: false,
		},
		{
			name:     "local mod pending",
			result:   Result{SyncLocalMod: 1},
			expected: true,
		},
		{
			name:     "remote mod pending",
			result:   Result{SyncRemoteMod: 2},
			expected: true,
		},
		{
			name:     "conflicts pending",
			result:   Result{SyncConflicts: 1},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasSyncPending(); got != tt.expected {
				t.Errorf("HasSyncPending() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResultSummary(t *testing.T) {
	syncedResult := Result{Action: ActionSynced}
	if syncedResult.Summary() != "All files are in sync" {
		t.Errorf("unexpected summary for synced: %s", syncedResult.Summary())
	}

	backedUpResult := Result{Action: ActionBackedUp, BackedUpCount: 3}
	if backedUpResult.Summary() != "Backed up 3 files" {
		t.Errorf("unexpected summary for backed up: %s", backedUpResult.Summary())
	}
}

func TestGetBackupFilesWithChanges(t *testing.T) {
	result := &DetectionResult{
		BackupFiles: []FileInfo{
			{State: StateSynced},
			{State: StateLocalModified},
			{State: StateLocalNew},
			{State: StateRemoteModified},
		},
	}

	changed := result.GetBackupFilesWithChanges()

	if len(changed) != 2 {
		t.Errorf("expected 2 changed backup files, got %d", len(changed))
	}
}

func TestGetSyncFilesWithChanges(t *testing.T) {
	result := &DetectionResult{
		SyncFiles: []FileInfo{
			{State: StateSynced},
			{State: StateLocalModified},
			{State: StateConflict},
		},
	}

	changed := result.GetSyncFilesWithChanges()

	if len(changed) != 2 {
		t.Errorf("expected 2 changed sync files, got %d", len(changed))
	}
}

func TestCountByMode(t *testing.T) {
	result := &DetectionResult{
		BackupFiles: []FileInfo{
			{State: StateSynced},
			{State: StateLocalModified},
		},
		SyncFiles: []FileInfo{
			{State: StateSynced},
			{State: StateConflict},
			{State: StateRemoteModified},
		},
	}

	backupChanged, syncChanged := result.CountByMode()

	if backupChanged != 1 {
		t.Errorf("expected 1 backup changed, got %d", backupChanged)
	}

	if syncChanged != 2 {
		t.Errorf("expected 2 sync changed, got %d", syncChanged)
	}
}

func TestGetAppIDs(t *testing.T) {
	result := &DetectionResult{
		LocalModFiles: []FileInfo{
			{AppID: "zsh"},
			{AppID: "git"},
		},
		RemoteModFiles: []FileInfo{
			{AppID: "zsh"}, // duplicate
			{AppID: "nvim"},
		},
	}

	appIDs := result.GetAppIDs()

	// Should have 3 unique app IDs
	if len(appIDs) != 3 {
		t.Errorf("expected 3 unique app IDs, got %d: %v", len(appIDs), appIDs)
	}
}

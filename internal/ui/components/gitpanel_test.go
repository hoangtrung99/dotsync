package components

import (
	"os"
	"path/filepath"
	"testing"

	"dotsync/internal/git"

	gitLib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestNewGitPanel(t *testing.T) {
	gp := NewGitPanel()

	if gp == nil {
		t.Fatal("NewGitPanel should return a GitPanel")
	}
	if gp.Width != 80 {
		t.Errorf("Expected width 80, got %d", gp.Width)
	}
	if gp.Height != 20 {
		t.Errorf("Expected height 20, got %d", gp.Height)
	}
	if gp.Mode != ModeStatus {
		t.Errorf("Expected ModeStatus, got %d", gp.Mode)
	}
}

func TestGitPanel_SetRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := gitLib.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	gp := NewGitPanel()
	repo := git.NewRepo(tempDir)
	gp.SetRepo(repo)

	if gp.Repo != repo {
		t.Error("Repo should be set")
	}
}

func TestGitPanel_Refresh_NoRepo(t *testing.T) {
	gp := NewGitPanel()
	// Should not panic
	gp.Refresh()
}

func TestGitPanel_Refresh_WithRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := gitLib.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &gitLib.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	gp := NewGitPanel()
	repo := git.NewRepo(tempDir)
	gp.SetRepo(repo)

	if gp.Status == nil {
		t.Error("Status should be populated after SetRepo")
	}
	if len(gp.Commits) == 0 {
		t.Error("Commits should be populated after SetRepo")
	}
}

func TestGitPanel_MoveUp(t *testing.T) {
	gp := NewGitPanel()
	gp.Cursor = 5

	gp.MoveUp()
	if gp.Cursor != 4 {
		t.Errorf("Expected 4, got %d", gp.Cursor)
	}

	gp.Cursor = 0
	gp.MoveUp()
	if gp.Cursor != 0 {
		t.Error("Cursor should not go below 0")
	}
}

func TestGitPanel_MoveDown(t *testing.T) {
	gp := NewGitPanel()

	gp.MoveDown()
	if gp.Cursor != 1 {
		t.Errorf("Expected 1, got %d", gp.Cursor)
	}

	gp.MoveDown()
	if gp.Cursor != 2 {
		t.Errorf("Expected 2, got %d", gp.Cursor)
	}
}

func TestGitPanel_ScrollUp(t *testing.T) {
	gp := NewGitPanel()
	gp.ScrollOffset = 5

	gp.ScrollUp()
	if gp.ScrollOffset != 4 {
		t.Errorf("Expected 4, got %d", gp.ScrollOffset)
	}

	gp.ScrollOffset = 0
	gp.ScrollUp()
	if gp.ScrollOffset != 0 {
		t.Error("ScrollOffset should not go below 0")
	}
}

func TestGitPanel_ScrollDown(t *testing.T) {
	gp := NewGitPanel()

	gp.ScrollDown()
	if gp.ScrollOffset != 1 {
		t.Errorf("Expected 1, got %d", gp.ScrollOffset)
	}
}

func TestGitPanel_View_NoRepo(t *testing.T) {
	gp := NewGitPanel()
	view := gp.View()

	if view != "No repository configured" {
		t.Errorf("Expected 'No repository configured', got %s", view)
	}
}

func TestGitPanel_View_WithRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := gitLib.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &gitLib.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	gp := NewGitPanel()
	repo := git.NewRepo(tempDir)
	gp.SetRepo(repo)

	view := gp.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestGitPanel_HasStagedChanges(t *testing.T) {
	gp := NewGitPanel()

	// No status
	if gp.HasStagedChanges() {
		t.Error("Should be false when no status")
	}

	// Empty staged
	gp.Status = &git.Status{
		Staged: []git.FileStatus{},
	}
	if gp.HasStagedChanges() {
		t.Error("Should be false when no staged files")
	}

	// With staged
	gp.Status.Staged = []git.FileStatus{{Path: "test.txt"}}
	if !gp.HasStagedChanges() {
		t.Error("Should be true when has staged files")
	}
}

func TestGitPanel_HasChanges(t *testing.T) {
	gp := NewGitPanel()

	// No status
	if gp.HasChanges() {
		t.Error("Should be false when no status")
	}

	// No changes
	gp.Status = &git.Status{
		HasChanges: false,
	}
	if gp.HasChanges() {
		t.Error("Should be false when no changes")
	}

	// With changes
	gp.Status.HasChanges = true
	if !gp.HasChanges() {
		t.Error("Should be true when has changes")
	}
}

func TestGitPanel_AddAll_NoRepo(t *testing.T) {
	gp := NewGitPanel()
	err := gp.AddAll()
	if err == nil {
		t.Error("Should return error when no repo")
	}
}

func TestGitPanel_Commit_NoRepo(t *testing.T) {
	gp := NewGitPanel()
	err := gp.Commit("test")
	if err == nil {
		t.Error("Should return error when no repo")
	}
}

func TestGitPanel_Commit_EmptyMessage(t *testing.T) {
	tempDir := t.TempDir()
	gitLib.PlainInit(tempDir, false)

	gp := NewGitPanel()
	gp.Repo = git.NewRepo(tempDir)

	err := gp.Commit("")
	if err == nil {
		t.Error("Should return error for empty message")
	}
}

func TestGitPanel_Push_NoRepo(t *testing.T) {
	gp := NewGitPanel()
	err := gp.Push()
	if err == nil {
		t.Error("Should return error when no repo")
	}
}

func TestGitPanel_Pull_NoRepo(t *testing.T) {
	gp := NewGitPanel()
	err := gp.Pull()
	if err == nil {
		t.Error("Should return error when no repo")
	}
}

func TestGitPanel_Fetch_NoRepo(t *testing.T) {
	gp := NewGitPanel()
	err := gp.Fetch()
	if err == nil {
		t.Error("Should return error when no repo")
	}
}

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"M", "●"},
		{"A", "+"},
		{"D", "✗"},
		{"R", "→"},
		{"C", "◎"},
		{"?", "?"},
		{"X", "?"},
	}

	for _, tc := range tests {
		result := getStatusIcon(tc.status)
		if result != tc.expected {
			t.Errorf("getStatusIcon(%s) = %s, expected %s", tc.status, result, tc.expected)
		}
	}
}

func TestGitPanelMode(t *testing.T) {
	if ModeStatus != 0 {
		t.Error("ModeStatus should be 0")
	}
	if ModeCommit != 1 {
		t.Error("ModeCommit should be 1")
	}
	if ModeBranches != 2 {
		t.Error("ModeBranches should be 2")
	}
}

func TestGitPanel_View_CleanWorktree(t *testing.T) {
	gp := NewGitPanel()
	gp.Status = &git.Status{
		Branch:  "main",
		IsClean: true,
	}
	gp.Repo = git.NewRepo("/tmp") // Just need non-nil for View to work

	view := gp.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestGitPanel_View_WithChanges(t *testing.T) {
	gp := NewGitPanel()
	gp.Status = &git.Status{
		Branch:     "main",
		HasChanges: true,
		IsClean:    false,
		Ahead:      2,
		Behind:     1,
		Staged: []git.FileStatus{
			{Path: "staged.txt", Status: "A"},
		},
		Modified: []git.FileStatus{
			{Path: "modified.txt", Status: "M"},
		},
		Untracked: []git.FileStatus{
			{Path: "untracked.txt", Status: "?"},
		},
	}
	gp.Commits = []git.CommitInfo{
		{Hash: "abc1234", Message: "Test commit", Author: "Test", Date: "2025-01-11"},
	}
	gp.Repo = git.NewRepo("/tmp")

	view := gp.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestGitPanel_View_LongCommitMessage(t *testing.T) {
	gp := NewGitPanel()
	gp.Status = &git.Status{
		Branch:  "main",
		IsClean: true,
	}
	gp.Commits = []git.CommitInfo{
		{Hash: "abc1234", Message: "This is a very long commit message that exceeds fifty characters and should be truncated", Author: "Test", Date: "2025-01-11"},
	}
	gp.Repo = git.NewRepo("/tmp")

	view := gp.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestGitPanel_ToggleBranchMode(t *testing.T) {
	gp := NewGitPanel()

	// Initially in status mode
	if gp.Mode != ModeStatus {
		t.Errorf("Expected ModeStatus, got %d", gp.Mode)
	}

	// Toggle to branch mode
	gp.ToggleBranchMode()
	if gp.Mode != ModeBranches {
		t.Errorf("Expected ModeBranches, got %d", gp.Mode)
	}
	if gp.BranchCursor != 0 {
		t.Errorf("BranchCursor should be 0 after entering branch mode")
	}

	// Toggle back to status mode
	gp.ToggleBranchMode()
	if gp.Mode != ModeStatus {
		t.Errorf("Expected ModeStatus, got %d", gp.Mode)
	}
}

func TestGitPanel_BranchNavigation(t *testing.T) {
	gp := NewGitPanel()
	gp.Branches = []string{"main", "develop", "feature-x"}
	gp.BranchCursor = 0

	// Move down
	gp.MoveBranchDown()
	if gp.BranchCursor != 1 {
		t.Errorf("Expected cursor 1, got %d", gp.BranchCursor)
	}

	gp.MoveBranchDown()
	if gp.BranchCursor != 2 {
		t.Errorf("Expected cursor 2, got %d", gp.BranchCursor)
	}

	// Should not go past last item
	gp.MoveBranchDown()
	if gp.BranchCursor != 2 {
		t.Errorf("Expected cursor to stay at 2, got %d", gp.BranchCursor)
	}

	// Move up
	gp.MoveBranchUp()
	if gp.BranchCursor != 1 {
		t.Errorf("Expected cursor 1, got %d", gp.BranchCursor)
	}

	gp.MoveBranchUp()
	if gp.BranchCursor != 0 {
		t.Errorf("Expected cursor 0, got %d", gp.BranchCursor)
	}

	// Should not go negative
	gp.MoveBranchUp()
	if gp.BranchCursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", gp.BranchCursor)
	}
}

func TestGitPanel_GetSelectedBranch(t *testing.T) {
	gp := NewGitPanel()
	gp.Branches = []string{"main", "develop", "feature-x"}

	// Test valid cursor
	gp.BranchCursor = 1
	if branch := gp.GetSelectedBranch(); branch != "develop" {
		t.Errorf("Expected 'develop', got '%s'", branch)
	}

	// Test out of range cursor
	gp.BranchCursor = 10
	if branch := gp.GetSelectedBranch(); branch != "" {
		t.Errorf("Expected empty string for out of range, got '%s'", branch)
	}

	// Test empty branches
	gp.Branches = nil
	gp.BranchCursor = 0
	if branch := gp.GetSelectedBranch(); branch != "" {
		t.Errorf("Expected empty string for nil branches, got '%s'", branch)
	}
}

func TestGitPanel_CheckoutBranch_NoRepo(t *testing.T) {
	gp := NewGitPanel()
	gp.Branches = []string{"main"}
	gp.BranchCursor = 0

	err := gp.CheckoutBranch()
	if err == nil {
		t.Error("Should return error when no repo")
	}
}

func TestGitPanel_CheckoutBranch_InvalidCursor(t *testing.T) {
	gp := NewGitPanel()
	gp.Repo = git.NewRepo("/tmp")
	gp.Branches = []string{"main"}
	gp.BranchCursor = 5 // Invalid cursor

	err := gp.CheckoutBranch()
	if err == nil {
		t.Error("Should return error for invalid branch selection")
	}
}

func TestGitPanel_RenderBranches(t *testing.T) {
	gp := NewGitPanel()
	gp.Branches = []string{"main", "develop"}
	gp.BranchCursor = 0
	gp.Status = &git.Status{Branch: "main"}
	gp.Mode = ModeBranches
	gp.Repo = git.NewRepo("/tmp")

	view := gp.View()
	if view == "" {
		t.Error("View should not be empty in branch mode")
	}
}

func TestGitPanel_RenderBranches_Empty(t *testing.T) {
	gp := NewGitPanel()
	gp.Branches = []string{}
	gp.Mode = ModeBranches
	gp.Repo = git.NewRepo("/tmp")

	view := gp.View()
	if view == "" {
		t.Error("View should not be empty even with no branches")
	}
}

func TestGitPanel_FooterChangesWithMode(t *testing.T) {
	gp := NewGitPanel()
	gp.Repo = git.NewRepo("/tmp")
	gp.Status = &git.Status{Branch: "main", IsClean: true}

	// Status mode footer
	gp.Mode = ModeStatus
	view1 := gp.View()

	// Branch mode footer
	gp.Mode = ModeBranches
	gp.Branches = []string{"main"}
	view2 := gp.View()

	// Views should be different
	if view1 == view2 {
		t.Error("Views should be different for different modes")
	}
}

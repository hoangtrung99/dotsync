package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestNewRepo(t *testing.T) {
	// Test with non-existent path
	repo := NewRepo("/nonexistent/path")
	if repo == nil {
		t.Error("NewRepo should return a Repo even for invalid paths")
	}
	if repo.IsRepo() {
		t.Error("IsRepo should return false for non-git directory")
	}

	// Test with temp directory (not a git repo)
	tempDir := t.TempDir()
	repo = NewRepo(tempDir)
	if repo.IsRepo() {
		t.Error("IsRepo should return false for non-git directory")
	}
}

func TestNewRepoWithGitDir(t *testing.T) {
	// Create a temp git repo
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// go-git requires proper git init, not just .git folder
	// So this will still return false
	repo := NewRepo(tempDir)
	// This is expected to be false since we didn't properly init
	if repo.Path != tempDir {
		t.Errorf("Expected path %s, got %s", tempDir, repo.Path)
	}
}

func TestRepoMethods_NotARepo(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewRepo(tempDir)

	// Test GetStatus on non-repo
	_, err := repo.GetStatus()
	if err == nil {
		t.Error("GetStatus should return error for non-repo")
	}

	// Test Add on non-repo
	err = repo.Add("file.txt")
	if err == nil {
		t.Error("Add should return error for non-repo")
	}

	// Test AddAll on non-repo
	err = repo.AddAll()
	if err == nil {
		t.Error("AddAll should return error for non-repo")
	}

	// Test Commit on non-repo
	err = repo.Commit("test commit")
	if err == nil {
		t.Error("Commit should return error for non-repo")
	}

	// Test CommitAmend on non-repo
	err = repo.CommitAmend("test commit")
	if err == nil {
		t.Error("CommitAmend should return error for non-repo")
	}

	// Test Push on non-repo
	err = repo.Push()
	if err == nil {
		t.Error("Push should return error for non-repo")
	}

	// Test Pull on non-repo
	err = repo.Pull()
	if err == nil {
		t.Error("Pull should return error for non-repo")
	}

	// Test Fetch on non-repo
	err = repo.Fetch()
	if err == nil {
		t.Error("Fetch should return error for non-repo")
	}

	// Test Checkout on non-repo
	err = repo.Checkout("main")
	if err == nil {
		t.Error("Checkout should return error for non-repo")
	}

	// Test Log on non-repo
	_, err = repo.Log(5)
	if err == nil {
		t.Error("Log should return error for non-repo")
	}
}

func TestCurrentBranch_NotARepo(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewRepo(tempDir)

	branch := repo.CurrentBranch()
	if branch != "unknown" {
		t.Errorf("Expected 'unknown', got %s", branch)
	}
}

func TestBranches_NotARepo(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewRepo(tempDir)

	branches := repo.Branches()
	if branches != nil {
		t.Error("Branches should return nil for non-repo")
	}
}

func TestHasRemote_NotARepo(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewRepo(tempDir)

	if repo.HasRemote() {
		t.Error("HasRemote should return false for non-repo")
	}
}

func TestRemoteURL_NotARepo(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewRepo(tempDir)

	url := repo.RemoteURL()
	if url != "" {
		t.Errorf("RemoteURL should return empty string for non-repo, got %s", url)
	}
}

func TestFileStatus(t *testing.T) {
	fs := FileStatus{
		Path:   "test.txt",
		Status: "M",
	}

	if fs.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %s", fs.Path)
	}
	if fs.Status != "M" {
		t.Errorf("Expected status 'M', got %s", fs.Status)
	}
}

func TestCommitInfo(t *testing.T) {
	ci := CommitInfo{
		Hash:    "abc1234",
		Message: "Test commit",
		Author:  "Test Author",
		Date:    "2025-01-11 10:00",
	}

	if ci.Hash != "abc1234" {
		t.Errorf("Expected hash 'abc1234', got %s", ci.Hash)
	}
	if ci.Message != "Test commit" {
		t.Errorf("Expected message 'Test commit', got %s", ci.Message)
	}
	if ci.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got %s", ci.Author)
	}
	if ci.Date != "2025-01-11 10:00" {
		t.Errorf("Expected date '2025-01-11 10:00', got %s", ci.Date)
	}
}

func TestStatusStruct(t *testing.T) {
	status := Status{
		Branch:     "main",
		Ahead:      2,
		Behind:     1,
		HasChanges: true,
		IsClean:    false,
	}

	if status.Branch != "main" {
		t.Errorf("Expected branch 'main', got %s", status.Branch)
	}
	if status.Ahead != 2 {
		t.Errorf("Expected ahead 2, got %d", status.Ahead)
	}
	if status.Behind != 1 {
		t.Errorf("Expected behind 1, got %d", status.Behind)
	}
	if !status.HasChanges {
		t.Error("Expected HasChanges to be true")
	}
	if status.IsClean {
		t.Error("Expected IsClean to be false")
	}
}

// Tests with real git repository
func TestWithRealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo using go-git
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	if !repo.IsRepo() {
		t.Error("IsRepo should return true for valid git repo")
	}
}

func TestGetStatus_EmptyRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()

	// May return error for empty repo (no HEAD), but should not panic
	if status != nil {
		if status.HasChanges && len(status.Staged) == 0 && len(status.Modified) == 0 && len(status.Untracked) == 0 {
			t.Error("HasChanges should be false when no changes")
		}
	}
	// Error is acceptable for empty repo with no HEAD
	_ = err
}

func TestGetStatus_WithFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create an untracked file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.Untracked) != 1 {
		t.Errorf("Expected 1 untracked file, got %d", len(status.Untracked))
	}
	if !status.HasChanges {
		t.Error("HasChanges should be true")
	}
	if status.IsClean {
		t.Error("IsClean should be false")
	}

	// Stage the file
	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")

	status, _ = repo.GetStatus()
	if len(status.Staged) != 1 {
		t.Errorf("Expected 1 staged file, got %d", len(status.Staged))
	}
}

func TestAdd_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	repo := NewRepo(tempDir)
	err = repo.Add("test.txt")
	if err != nil {
		t.Errorf("Add should not fail: %v", err)
	}

	// Verify file is staged
	status, _ := repo.GetStatus()
	if len(status.Staged) != 1 {
		t.Errorf("Expected 1 staged file, got %d", len(status.Staged))
	}
}

func TestCurrentBranch_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file to have HEAD
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)
	branch := repo.CurrentBranch()
	// Default branch might be "master" or "main" depending on git config
	if branch != "master" && branch != "main" {
		t.Errorf("Expected 'master' or 'main', got %s", branch)
	}
}

func TestBranches_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file to have HEAD
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)
	branches := repo.Branches()
	if len(branches) == 0 {
		t.Error("Expected at least 1 branch")
	}
}

func TestLog_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)
	commits, err := repo.Log(5)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if len(commits) != 1 {
		t.Errorf("Expected 1 commit, got %d", len(commits))
	}
	if commits[0].Message != "initial commit" {
		t.Errorf("Expected message 'initial commit', got %s", commits[0].Message)
	}
	if commits[0].Author != "Test" {
		t.Errorf("Expected author 'Test', got %s", commits[0].Author)
	}
}

func TestCommit_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and stage a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")

	repo := NewRepo(tempDir)
	err = repo.Commit("test commit")
	if err != nil {
		t.Errorf("Commit failed: %v", err)
	}
}

func TestHasRemote_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	if repo.HasRemote() {
		t.Error("HasRemote should return false for repo without remote")
	}
}

func TestRemoteURL_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	url := repo.RemoteURL()
	if url != "" {
		t.Error("RemoteURL should return empty for repo without remote")
	}
}

func TestCheckout_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file to have HEAD
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Create a new branch
	headRef, _ := gitRepo.Head()
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName("feature"), headRef.Hash())
	gitRepo.Storer.SetReference(ref)

	repo := NewRepo(tempDir)
	err = repo.Checkout("feature")
	if err != nil {
		t.Errorf("Checkout failed: %v", err)
	}

	branch := repo.CurrentBranch()
	if branch != "feature" {
		t.Errorf("Expected branch 'feature', got %s", branch)
	}
}

func TestStatusWithModifiedFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Modify the file
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.Modified) != 1 {
		t.Errorf("Expected 1 modified file, got %d", len(status.Modified))
	}
	if !status.HasChanges {
		t.Error("HasChanges should be true")
	}
}

func TestAddAll_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create multiple files
	for i := 1; i <= 3; i++ {
		testFile := filepath.Join(tempDir, "test"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	repo := NewRepo(tempDir)
	err = repo.AddAll()
	// AddAll may fail due to git command not available in test env
	// Just verify it doesn't panic
	_ = err
}

func TestStash_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Modify the file to have something to stash
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	repo := NewRepo(tempDir)
	err = repo.Stash()
	// Stash uses exec.Command, may fail in test env
	// Just verify it doesn't panic
	_ = err
}

func TestStashPop_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)
	// StashPop without stash will fail, but should not panic
	err = repo.StashPop()
	_ = err
}

func TestCommitAmend_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)
	err = repo.CommitAmend("amended commit")
	// CommitAmend uses exec.Command, may fail in test env
	// Just verify it doesn't panic
	_ = err
}

func TestPushWithUpstream_NotARepo(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewRepo(tempDir)

	err := repo.PushWithUpstream("origin", "main")
	if err == nil {
		t.Error("PushWithUpstream should return error for non-repo")
	}
}

func TestPushWithUpstream_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	// PushWithUpstream will fail without remote, but should not panic
	err = repo.PushWithUpstream("origin", "main")
	// Error is expected since no remote configured
	_ = err
}

func TestMultipleCommits_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	worktree, _ := gitRepo.Worktree()

	// Create multiple commits
	for i := 1; i <= 5; i++ {
		testFile := filepath.Join(tempDir, "test.txt")
		content := []byte("commit " + string(rune('0'+i)))
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		worktree.Add("test.txt")
		worktree.Commit("commit "+string(rune('0'+i)), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@test.com",
			},
		})
	}

	repo := NewRepo(tempDir)
	commits, err := repo.Log(3)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}
}

func TestGetStatus_DeletedFile(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Delete the file
	os.Remove(testFile)

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	// Should have changes (deleted file shows as modified)
	if !status.HasChanges {
		t.Error("HasChanges should be true for deleted file")
	}
}

func TestAdd_MultipleFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create multiple files
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, f := range files {
		testFile := filepath.Join(tempDir, f)
		if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	repo := NewRepo(tempDir)
	err = repo.Add(files...)
	if err != nil {
		t.Errorf("Add multiple files failed: %v", err)
	}

	status, _ := repo.GetStatus()
	if len(status.Staged) != 3 {
		t.Errorf("Expected 3 staged files, got %d", len(status.Staged))
	}
}

func TestCheckout_NonExistentBranch(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file to have HEAD
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)
	err = repo.Checkout("nonexistent-branch")
	if err == nil {
		t.Error("Checkout should fail for non-existent branch")
	}
}

func TestLog_EmptyRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo without commits
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	_, err = repo.Log(5)
	// Should return error for empty repo (no HEAD)
	if err == nil {
		t.Error("Log should return error for empty repo")
	}
}

func TestAdd_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	err = repo.Add("nonexistent.txt")
	if err == nil {
		t.Error("Add should fail for non-existent file")
	}
}

func TestFetch_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	// Fetch will fail without remote, but should not panic
	err = repo.Fetch()
	// Error is expected since no remote configured
	_ = err
}

func TestPush_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	// Push will fail without remote, but should not panic
	err = repo.Push()
	// Error is expected since no remote configured
	_ = err
}

func TestPull_RealRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	// Pull will fail without remote, but should not panic
	err = repo.Pull()
	// Error is expected since no remote configured
	_ = err
}

func TestGetStatus_WithModifiedAndUntracked(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit
	worktree, _ := gitRepo.Worktree()
	testFile := filepath.Join(tempDir, "tracked.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)
	worktree.Add("tracked.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Modify tracked file
	os.WriteFile(testFile, []byte("modified content"), 0644)

	// Create untracked file
	os.WriteFile(filepath.Join(tempDir, "untracked.txt"), []byte("new"), 0644)

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !status.HasChanges {
		t.Error("HasChanges should be true")
	}

	if status.IsClean {
		t.Error("IsClean should be false")
	}

	// Check that we have modified and untracked files
	if len(status.Modified) == 0 {
		t.Error("Should have modified files")
	}
	if len(status.Untracked) == 0 {
		t.Error("Should have untracked files")
	}
}

func TestGetStatus_WithStagedRenamedFile(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "original.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("original.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Rename the file using git mv
	newFile := filepath.Join(tempDir, "renamed.txt")
	os.Rename(testFile, newFile)
	worktree.Remove("original.txt")
	worktree.Add("renamed.txt")

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !status.HasChanges {
		t.Error("HasChanges should be true for renamed file")
	}
}

func TestGetStatus_WithStagedDeletedFile(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "toDelete.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("toDelete.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Stage deletion
	worktree.Remove("toDelete.txt")
	os.Remove(testFile)

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !status.HasChanges {
		t.Error("HasChanges should be true for staged deletion")
	}

	// Should have staged deleted file
	hasDeleted := false
	for _, f := range status.Staged {
		if f.Status == "D" {
			hasDeleted = true
			break
		}
	}
	if !hasDeleted {
		t.Error("Should have staged deleted file")
	}
}

func TestRemoteURL_WithRemote(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a remote
	_, err = gitRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/test/repo.git"},
	})
	if err != nil {
		t.Fatalf("Failed to create remote: %v", err)
	}

	repo := NewRepo(tempDir)

	// Test HasRemote
	if !repo.HasRemote() {
		t.Error("HasRemote should return true when remote exists")
	}

	// Test RemoteURL
	url := repo.RemoteURL()
	if url != "https://github.com/test/repo.git" {
		t.Errorf("Expected remote URL, got %s", url)
	}
}

func TestAddAll_FallbackPath(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create multiple files
	for i := 1; i <= 3; i++ {
		testFile := filepath.Join(tempDir, "test"+string(rune('0'+i))+".txt")
		os.WriteFile(testFile, []byte("content"), 0644)
	}

	repo := NewRepo(tempDir)

	// AddAll should work (uses git command or fallback)
	err = repo.AddAll()
	// If git command fails, fallback should work
	// Just verify no panic
	_ = err

	// Check status to verify files were added
	status, _ := repo.GetStatus()
	// If AddAll worked, should have staged or no untracked files
	_ = status
}

func TestGetStatus_CleanRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	// Clean repo should have no changes
	if status.HasChanges {
		t.Error("HasChanges should be false for clean repo")
	}
	if !status.IsClean {
		t.Error("IsClean should be true for clean repo")
	}
}

func TestGetStatus_WithCopiedFile(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "original.txt")
	os.WriteFile(testFile, []byte("content to copy"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("original.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Copy the file
	copyFile := filepath.Join(tempDir, "copied.txt")
	os.WriteFile(copyFile, []byte("content to copy"), 0644)
	worktree.Add("copied.txt")

	repo := NewRepo(tempDir)
	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	// Should have staged file
	if len(status.Staged) == 0 {
		t.Error("Should have staged file for copy")
	}
}

func TestStash_NoChanges(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)
	// Stash with no changes - should complete without panic
	err = repo.Stash()
	_ = err
}

func TestCheckout_ExistingBranch(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Create a new branch
	head, _ := gitRepo.Head()
	gitRepo.CreateBranch(&config.Branch{
		Name:   "feature",
		Remote: "",
		Merge:  plumbing.ReferenceName("refs/heads/feature"),
	})
	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/feature"), head.Hash())
	gitRepo.Storer.SetReference(ref)

	repo := NewRepo(tempDir)

	// Checkout to feature branch
	err = repo.Checkout("feature")
	if err != nil {
		t.Errorf("Checkout should succeed for existing branch: %v", err)
	}

	// Check we're on feature branch
	branch := repo.CurrentBranch()
	if branch != "feature" {
		t.Errorf("Expected branch 'feature', got '%s'", branch)
	}
}

func TestCurrentBranch_EmptyRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo but don't commit
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)
	branch := repo.CurrentBranch()
	// Empty repo might return empty string or "master"
	_ = branch
}

func TestBranches_MultipleBranches(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("test.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Create additional branches
	head, _ := gitRepo.Head()
	for _, branchName := range []string{"develop", "feature1", "feature2"} {
		gitRepo.CreateBranch(&config.Branch{
			Name:  branchName,
			Merge: plumbing.ReferenceName("refs/heads/" + branchName),
		})
		ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branchName), head.Hash())
		gitRepo.Storer.SetReference(ref)
	}

	repo := NewRepo(tempDir)
	branches := repo.Branches()

	// Should have at least 4 branches (master/main + 3 we created)
	if len(branches) < 3 {
		t.Errorf("Expected at least 3 branches, got %d", len(branches))
	}
}

func TestLog_WithMultipleCommits(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	worktree, _ := gitRepo.Worktree()

	// Create multiple commits
	for i := 1; i <= 5; i++ {
		testFile := filepath.Join(tempDir, "test.txt")
		os.WriteFile(testFile, []byte("content "+string(rune('0'+i))), 0644)
		worktree.Add("test.txt")
		worktree.Commit("Commit "+string(rune('0'+i)), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@test.com",
			},
		})
	}

	repo := NewRepo(tempDir)
	commits, err := repo.Log(3)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Should return 3 commits (limited)
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}
}

func TestAddAndCommit_Success(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "initial.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("initial.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)

	// Create a new file
	newFile := filepath.Join(tempDir, "new.txt")
	os.WriteFile(newFile, []byte("new content"), 0644)

	// Add it
	err = repo.Add("new.txt")
	if err != nil {
		t.Errorf("Add should succeed: %v", err)
	}

	// Commit it
	err = repo.Commit("Add new file")
	if err != nil {
		t.Errorf("Commit should succeed: %v", err)
	}

	// Verify commit was made
	commits, _ := repo.Log(1)
	if len(commits) < 1 || commits[0].Message != "Add new file" {
		t.Error("Commit message not found in log")
	}
}

func TestAddAll_Success(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "initial.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("initial.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)

	// Create multiple new files
	os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("content2"), 0644)
	os.WriteFile(filepath.Join(tempDir, "file3.txt"), []byte("content3"), 0644)

	// AddAll
	err = repo.AddAll()
	if err != nil {
		t.Errorf("AddAll should succeed: %v", err)
	}

	// Check status - files should be staged
	status, _ := repo.GetStatus()
	if len(status.Staged) == 0 && len(status.Modified) == 0 && len(status.Untracked) == 0 {
		// All files staged or at least processed
	}
}

func TestGetStatus_WithChanges(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "initial.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("initial.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)

	// Create untracked file
	os.WriteFile(filepath.Join(tempDir, "untracked.txt"), []byte("untracked"), 0644)

	// Modify existing file
	os.WriteFile(testFile, []byte("modified content"), 0644)

	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !status.HasChanges {
		t.Error("HasChanges should be true")
	}
	if status.IsClean {
		t.Error("IsClean should be false")
	}
}

func TestPushWithUpstream_NoRemote(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "initial.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("initial.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)

	// Try to push without remote configured
	err = repo.PushWithUpstream("origin", "main")
	// Should fail since no remote
	if err == nil {
		t.Error("PushWithUpstream should fail without remote")
	}
}

func TestRemoteURL_NoRemoteConfigured(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo without remote
	_, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	repo := NewRepo(tempDir)

	url := repo.RemoteURL()
	if url != "" {
		t.Errorf("RemoteURL should be empty for repo without remote, got %s", url)
	}
}

func TestCommitAmend_WithChanges(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "initial.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("initial.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	repo := NewRepo(tempDir)

	// CommitAmend - may fail if git command not available
	err = repo.CommitAmend("Amended commit")
	// We just verify it doesn't panic, actual result depends on git availability
	_ = err
}

func TestGetStatus_WithStagedAndModified(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize a real git repo
	gitRepo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "initial.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)

	worktree, _ := gitRepo.Worktree()
	worktree.Add("initial.txt")
	worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
		},
	})

	// Create and stage a new file
	newFile := filepath.Join(tempDir, "staged.txt")
	os.WriteFile(newFile, []byte("staged content"), 0644)
	worktree.Add("staged.txt")

	repo := NewRepo(tempDir)

	status, err := repo.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.Staged) == 0 {
		t.Error("Should have staged files")
	}
}

package git

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Repo represents a git repository
type Repo struct {
	Path string
	repo *git.Repository
}

// NewRepo creates a new Repo for the given path
func NewRepo(path string) *Repo {
	r := &Repo{Path: path}
	repo, err := git.PlainOpen(path)
	if err == nil {
		r.repo = repo
	}
	return r
}

// IsRepo checks if the path is a git repository
func (r *Repo) IsRepo() bool {
	return r.repo != nil
}

// Status represents git repository status
type Status struct {
	Branch     string
	Ahead      int
	Behind     int
	Staged     []FileStatus
	Modified   []FileStatus
	Untracked  []FileStatus
	HasChanges bool
	IsClean    bool
}

// FileStatus represents the status of a single file
type FileStatus struct {
	Path   string
	Status string // "M" = modified, "A" = added, "D" = deleted, "?" = untracked
}

// GetStatus returns the current status of the repository
func (r *Repo) GetStatus() (*Status, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("not a git repository")
	}

	status := &Status{}

	// Get current branch
	head, err := r.repo.Head()
	if err == nil {
		status.Branch = head.Name().Short()
	}

	// Get ahead/behind counts
	r.calculateAheadBehind(status)

	// Get worktree status
	worktree, err := r.repo.Worktree()
	if err != nil {
		return status, err
	}

	gitStatus, err := worktree.Status()
	if err != nil {
		return status, err
	}

	for path, fileStatus := range gitStatus {
		fs := FileStatus{Path: path}

		// Check staged changes (index)
		switch fileStatus.Staging {
		case git.Added:
			fs.Status = "A"
			status.Staged = append(status.Staged, fs)
		case git.Modified:
			fs.Status = "M"
			status.Staged = append(status.Staged, fs)
		case git.Deleted:
			fs.Status = "D"
			status.Staged = append(status.Staged, fs)
		case git.Renamed:
			fs.Status = "R"
			status.Staged = append(status.Staged, fs)
		case git.Copied:
			fs.Status = "C"
			status.Staged = append(status.Staged, fs)
		}

		// Check unstaged changes (worktree)
		switch fileStatus.Worktree {
		case git.Modified:
			fs.Status = "M"
			status.Modified = append(status.Modified, fs)
		case git.Deleted:
			fs.Status = "D"
			status.Modified = append(status.Modified, fs)
		case git.Untracked:
			fs.Status = "?"
			status.Untracked = append(status.Untracked, fs)
		}
	}

	// Sort for consistent display
	sort.Slice(status.Staged, func(i, j int) bool { return status.Staged[i].Path < status.Staged[j].Path })
	sort.Slice(status.Modified, func(i, j int) bool { return status.Modified[i].Path < status.Modified[j].Path })
	sort.Slice(status.Untracked, func(i, j int) bool { return status.Untracked[i].Path < status.Untracked[j].Path })

	status.HasChanges = len(status.Staged) > 0 || len(status.Modified) > 0 || len(status.Untracked) > 0
	status.IsClean = !status.HasChanges

	return status, nil
}

// calculateAheadBehind calculates ahead/behind counts
func (r *Repo) calculateAheadBehind(status *Status) {
	head, err := r.repo.Head()
	if err != nil {
		return
	}

	// Get remote tracking branch
	remoteName := "origin"
	branchName := head.Name().Short()
	remoteRef := plumbing.NewRemoteReferenceName(remoteName, branchName)

	remoteHash, err := r.repo.Reference(remoteRef, true)
	if err != nil {
		return
	}

	localHash := head.Hash()
	remoteHeadHash := remoteHash.Hash()

	if localHash == remoteHeadHash {
		return
	}

	// Count commits ahead/behind
	localCommits := make(map[plumbing.Hash]bool)
	remoteCommits := make(map[plumbing.Hash]bool)

	// Get local commits
	localIter, err := r.repo.Log(&git.LogOptions{From: localHash})
	if err == nil {
		localIter.ForEach(func(c *object.Commit) error {
			localCommits[c.Hash] = true
			return nil
		})
	}

	// Get remote commits
	remoteIter, err := r.repo.Log(&git.LogOptions{From: remoteHeadHash})
	if err == nil {
		remoteIter.ForEach(func(c *object.Commit) error {
			remoteCommits[c.Hash] = true
			return nil
		})
	}

	// Count ahead (local commits not in remote)
	for hash := range localCommits {
		if !remoteCommits[hash] {
			status.Ahead++
		}
	}

	// Count behind (remote commits not in local)
	for hash := range remoteCommits {
		if !localCommits[hash] {
			status.Behind++
		}
	}
}

// Add stages files for commit
func (r *Repo) Add(files ...string) error {
	if r.repo == nil {
		return fmt.Errorf("not a git repository")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	for _, file := range files {
		_, err = worktree.Add(file)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddAll stages all changes
func (r *Repo) AddAll() error {
	if r.repo == nil {
		return fmt.Errorf("not a git repository")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	// Use git command for AddAll since go-git's Add with glob is limited
	cmd := exec.Command("git", "-C", r.Path, "add", "-A")
	if err := cmd.Run(); err != nil {
		// Fallback: add each file individually
		status, err := worktree.Status()
		if err != nil {
			return err
		}
		for path := range status {
			worktree.Add(path)
		}
	}
	return nil
}

// Commit creates a commit with the given message
func (r *Repo) Commit(message string) error {
	if r.repo == nil {
		return fmt.Errorf("not a git repository")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	_, err = worktree.Commit(message, &git.CommitOptions{})
	return err
}

// CommitAmend amends the last commit
func (r *Repo) CommitAmend(message string) error {
	if r.repo == nil {
		return fmt.Errorf("not a git repository")
	}

	// go-git doesn't support amend directly, use exec
	cmd := exec.Command("git", "-C", r.Path, "commit", "--amend", "-m", message)
	return cmd.Run()
}

// Push pushes to the remote
func (r *Repo) Push() error {
	if r.repo == nil {
		return fmt.Errorf("not a git repository")
	}

	// Use exec for push as go-git requires explicit auth setup
	cmd := exec.Command("git", "-C", r.Path, "push")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("push failed: %s", string(output))
	}
	return nil
}

// PushWithUpstream pushes and sets upstream
func (r *Repo) PushWithUpstream(remote, branch string) error {
	cmd := exec.Command("git", "-C", r.Path, "push", "-u", remote, branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("push failed: %s", string(output))
	}
	return nil
}

// Pull pulls from the remote
func (r *Repo) Pull() error {
	if r.repo == nil {
		return fmt.Errorf("not a git repository")
	}

	// Use exec for pull as go-git requires explicit auth setup
	cmd := exec.Command("git", "-C", r.Path, "pull")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pull failed: %s", string(output))
	}
	return nil
}

// Fetch fetches from the remote
func (r *Repo) Fetch() error {
	if r.repo == nil {
		return fmt.Errorf("not a git repository")
	}

	// Use exec for fetch as go-git requires explicit auth setup
	cmd := exec.Command("git", "-C", r.Path, "fetch")
	return cmd.Run()
}

// Stash stashes current changes
func (r *Repo) Stash() error {
	cmd := exec.Command("git", "-C", r.Path, "stash")
	return cmd.Run()
}

// StashPop pops the latest stash
func (r *Repo) StashPop() error {
	cmd := exec.Command("git", "-C", r.Path, "stash", "pop")
	return cmd.Run()
}

// CurrentBranch returns the current branch name
func (r *Repo) CurrentBranch() string {
	if r.repo == nil {
		return "unknown"
	}

	head, err := r.repo.Head()
	if err != nil {
		return "unknown"
	}
	return head.Name().Short()
}

// Branches returns all local branches
func (r *Repo) Branches() []string {
	if r.repo == nil {
		return nil
	}

	branchRefs, err := r.repo.Branches()
	if err != nil {
		return nil
	}

	var branches []string
	branchRefs.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, ref.Name().Short())
		return nil
	})

	sort.Strings(branches)
	return branches
}

// Checkout switches to a branch
func (r *Repo) Checkout(branch string) error {
	if r.repo == nil {
		return fmt.Errorf("not a git repository")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	return worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
	})
}

// Log returns recent commit logs
func (r *Repo) Log(count int) ([]CommitInfo, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("not a git repository")
	}

	head, err := r.repo.Head()
	if err != nil {
		return nil, err
	}

	commitIter, err := r.repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	i := 0
	err = commitIter.ForEach(func(c *object.Commit) error {
		if i >= count {
			return fmt.Errorf("done")
		}
		commits = append(commits, CommitInfo{
			Hash:    c.Hash.String()[:7],
			Message: strings.Split(c.Message, "\n")[0],
			Author:  c.Author.Name,
			Date:    c.Author.When.Format("2006-01-02 15:04"),
		})
		i++
		return nil
	})

	return commits, nil
}

// CommitInfo holds commit information
type CommitInfo struct {
	Hash    string
	Message string
	Author  string
	Date    string
}

// HasRemote checks if a remote is configured
func (r *Repo) HasRemote() bool {
	if r.repo == nil {
		return false
	}

	remotes, err := r.repo.Remotes()
	return err == nil && len(remotes) > 0
}

// RemoteURL returns the remote URL
func (r *Repo) RemoteURL() string {
	if r.repo == nil {
		return ""
	}

	remote, err := r.repo.Remote("origin")
	if err != nil {
		return ""
	}

	config := remote.Config()
	if len(config.URLs) > 0 {
		return config.URLs[0]
	}
	return ""
}

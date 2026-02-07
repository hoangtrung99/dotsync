package components

import (
	"fmt"
	"strings"

	"dotsync/internal/git"
	"dotsync/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// GitPanel displays git operations UI
type GitPanel struct {
	Width  int
	Height int

	Repo     *git.Repo
	Status   *git.Status
	Commits  []git.CommitInfo
	Branches []string

	Cursor       int
	ScrollOffset int
	Mode         GitPanelMode
	BranchCursor int

	// Commit message input
	CommitMessage string

	// Styles
	headerStyle    lipgloss.Style
	stagedStyle    lipgloss.Style
	modifiedStyle  lipgloss.Style
	untrackedStyle lipgloss.Style
	branchStyle    lipgloss.Style
}

// GitPanelMode represents the current mode of the git panel
type GitPanelMode int

const (
	ModeStatus GitPanelMode = iota
	ModeCommit
	ModeBranches
)

// NewGitPanel creates a new GitPanel
func NewGitPanel() *GitPanel {
	return &GitPanel{
		Width:  80,
		Height: 20,
		Mode:   ModeStatus,
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#89b4fa")),
		stagedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e3a1")),
		modifiedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f9e2af")),
		untrackedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086")),
		branchStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cba6f7")).
			Bold(true),
	}
}

// SetRepo sets the git repository
func (g *GitPanel) SetRepo(repo *git.Repo) {
	g.Repo = repo
	g.Refresh()
}

// Refresh refreshes the git status
func (g *GitPanel) Refresh() {
	if g.Repo == nil {
		return
	}

	status, err := g.Repo.GetStatus()
	if err == nil {
		g.Status = status
	}

	commits, err := g.Repo.Log(5)
	if err == nil {
		g.Commits = commits
	}

	// Load branches
	g.Branches = g.Repo.Branches()
}

// MoveUp moves cursor up
func (g *GitPanel) MoveUp() {
	if g.Cursor > 0 {
		g.Cursor--
	}
}

// MoveDown moves cursor down
func (g *GitPanel) MoveDown() {
	g.Cursor++
}

// ScrollUp scrolls view up
func (g *GitPanel) ScrollUp() {
	if g.ScrollOffset > 0 {
		g.ScrollOffset--
	}
}

// ScrollDown scrolls view down
func (g *GitPanel) ScrollDown() {
	g.ScrollOffset++
}

// View renders the git panel
func (g *GitPanel) View() string {
	if g.Repo == nil {
		return "No repository configured"
	}

	var b strings.Builder

	// Header
	header := g.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Content based on mode
	switch g.Mode {
	case ModeBranches:
		b.WriteString(g.renderBranches())
	default:
		// Status section
		statusSection := g.renderStatus()
		b.WriteString(statusSection)
		b.WriteString("\n")

		// Recent commits
		commitsSection := g.renderCommits()
		b.WriteString(commitsSection)
	}

	b.WriteString("\n")

	// Footer
	b.WriteString(g.renderFooter())

	return b.String()
}

func (g *GitPanel) renderHeader() string {
	title := g.headerStyle.Render("🔀 Git Operations")

	branch := "unknown"
	if g.Status != nil {
		branch = g.Status.Branch
	}

	branchInfo := g.branchStyle.Render("⎇ " + branch)

	// Ahead/Behind info
	var syncInfo string
	if g.Status != nil {
		if g.Status.Ahead > 0 {
			syncInfo += fmt.Sprintf(" ↑%d", g.Status.Ahead)
		}
		if g.Status.Behind > 0 {
			syncInfo += fmt.Sprintf(" ↓%d", g.Status.Behind)
		}
	}

	return fmt.Sprintf("%s  %s%s", title, branchInfo, ui.MutedStyle.Render(syncInfo))
}

func (g *GitPanel) renderStatus() string {
	var b strings.Builder

	b.WriteString(ui.PanelTitleStyle.Render("Changes"))
	b.WriteString("\n")

	if g.Status == nil {
		b.WriteString(ui.MutedStyle.Render("  Loading..."))
		return b.String()
	}

	if g.Status.IsClean {
		b.WriteString(g.stagedStyle.Render("  ✓ Working tree clean"))
		return b.String()
	}

	// Staged files
	if len(g.Status.Staged) > 0 {
		b.WriteString(g.stagedStyle.Render("  Staged:\n"))
		for _, f := range g.Status.Staged {
			icon := getStatusIcon(f.Status)
			b.WriteString(fmt.Sprintf("    %s %s\n", icon, f.Path))
		}
	}

	// Modified files
	if len(g.Status.Modified) > 0 {
		b.WriteString(g.modifiedStyle.Render("  Modified:\n"))
		for _, f := range g.Status.Modified {
			icon := getStatusIcon(f.Status)
			b.WriteString(fmt.Sprintf("    %s %s\n", icon, f.Path))
		}
	}

	// Untracked files
	if len(g.Status.Untracked) > 0 {
		b.WriteString(g.untrackedStyle.Render("  Untracked:\n"))
		for _, f := range g.Status.Untracked {
			b.WriteString(fmt.Sprintf("    ? %s\n", f.Path))
		}
	}

	return b.String()
}

func (g *GitPanel) renderCommits() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(ui.PanelTitleStyle.Render("Recent Commits"))
	b.WriteString("\n")

	if len(g.Commits) == 0 {
		b.WriteString(ui.MutedStyle.Render("  No commits yet"))
		return b.String()
	}

	for _, commit := range g.Commits {
		hash := ui.MutedStyle.Render(commit.Hash)
		msg := commit.Message
		if len(msg) > 50 {
			msg = msg[:47] + "..."
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", hash, msg))
	}

	return b.String()
}

func (g *GitPanel) renderFooter() string {
	var items []string

	switch g.Mode {
	case ModeBranches:
		items = []string{
			ui.RenderHelpItem("↑/↓", "navigate"),
			ui.RenderHelpItem("Enter", "checkout"),
			ui.RenderHelpItem("b", "back to status"),
			ui.RenderHelpItem("ESC", "close"),
		}
	default:
		// Highlight push if there are commits ahead
		pushLabel := "push"
		if g.Status != nil && g.Status.Ahead > 0 {
			pushLabel = fmt.Sprintf("push ↑%d", g.Status.Ahead)
		}

		items = []string{
			ui.RenderHelpItem("a", "add all"),
			ui.RenderHelpItem("c", "commit"),
			ui.RenderHelpItem("p", pushLabel),
			ui.RenderHelpItem("f", "fetch"),
			ui.RenderHelpItem("l", "pull"),
			ui.RenderHelpItem("s", "stash"),
			ui.RenderHelpItem("b", "branches"),
			ui.RenderHelpItem("L", "lazygit"),
			ui.RenderHelpItem("r", "refresh"),
			ui.RenderHelpItem("ESC", "back"),
		}
	}

	return ui.HelpBarStyle.Render(strings.Join(items, "  "))
}

func getStatusIcon(status string) string {
	switch status {
	case "M":
		return "●"
	case "A":
		return "+"
	case "D":
		return "✗"
	case "R":
		return "→"
	case "C":
		return "◎"
	default:
		return "?"
	}
}

// Actions

// AddAll stages all changes
func (g *GitPanel) AddAll() error {
	if g.Repo == nil {
		return fmt.Errorf("no repository")
	}
	err := g.Repo.AddAll()
	if err == nil {
		g.Refresh()
	}
	return err
}

// Commit commits staged changes
func (g *GitPanel) Commit(message string) error {
	if g.Repo == nil {
		return fmt.Errorf("no repository")
	}
	if message == "" {
		return fmt.Errorf("commit message is required")
	}
	err := g.Repo.Commit(message)
	if err == nil {
		g.Refresh()
	}
	return err
}

// Push pushes to remote
func (g *GitPanel) Push() error {
	if g.Repo == nil {
		return fmt.Errorf("no repository")
	}
	err := g.Repo.Push()
	if err == nil {
		g.Refresh()
	}
	return err
}

// Pull pulls from remote
func (g *GitPanel) Pull() error {
	if g.Repo == nil {
		return fmt.Errorf("no repository")
	}
	err := g.Repo.Pull()
	if err == nil {
		g.Refresh()
	}
	return err
}

// Fetch fetches from remote
func (g *GitPanel) Fetch() error {
	if g.Repo == nil {
		return fmt.Errorf("no repository")
	}
	err := g.Repo.Fetch()
	if err == nil {
		g.Refresh()
	}
	return err
}

// HasStagedChanges returns true if there are staged changes
func (g *GitPanel) HasStagedChanges() bool {
	return g.Status != nil && len(g.Status.Staged) > 0
}

// HasChanges returns true if there are any changes
func (g *GitPanel) HasChanges() bool {
	return g.Status != nil && g.Status.HasChanges
}

// Stash stashes current changes
func (g *GitPanel) Stash() error {
	if g.Repo == nil {
		return fmt.Errorf("no repository")
	}
	err := g.Repo.Stash()
	if err == nil {
		g.Refresh()
	}
	return err
}

// StashPop pops the latest stash
func (g *GitPanel) StashPop() error {
	if g.Repo == nil {
		return fmt.Errorf("no repository")
	}
	err := g.Repo.StashPop()
	if err == nil {
		g.Refresh()
	}
	return err
}

// ToggleBranchMode toggles between status and branch mode
func (g *GitPanel) ToggleBranchMode() {
	if g.Mode == ModeBranches {
		g.Mode = ModeStatus
	} else {
		g.Mode = ModeBranches
		g.BranchCursor = 0
	}
}

// MoveBranchUp moves branch cursor up
func (g *GitPanel) MoveBranchUp() {
	if g.BranchCursor > 0 {
		g.BranchCursor--
	}
}

// MoveBranchDown moves branch cursor down
func (g *GitPanel) MoveBranchDown() {
	if g.BranchCursor < len(g.Branches)-1 {
		g.BranchCursor++
	}
}

// CheckoutBranch checks out the selected branch
func (g *GitPanel) CheckoutBranch() error {
	if g.Repo == nil {
		return fmt.Errorf("no repository")
	}
	if g.BranchCursor >= len(g.Branches) {
		return fmt.Errorf("invalid branch selection")
	}
	branch := g.Branches[g.BranchCursor]
	err := g.Repo.Checkout(branch)
	if err == nil {
		g.Refresh()
		g.Mode = ModeStatus
	}
	return err
}

// GetSelectedBranch returns the currently selected branch name
func (g *GitPanel) GetSelectedBranch() string {
	if g.BranchCursor >= len(g.Branches) {
		return ""
	}
	return g.Branches[g.BranchCursor]
}

func (g *GitPanel) renderBranches() string {
	var b strings.Builder

	b.WriteString(ui.PanelTitleStyle.Render("Branches"))
	b.WriteString("\n\n")

	if len(g.Branches) == 0 {
		b.WriteString(ui.MutedStyle.Render("  No branches found"))
		return b.String()
	}

	currentBranch := ""
	if g.Status != nil {
		currentBranch = g.Status.Branch
	}

	for i, branch := range g.Branches {
		prefix := "  "
		if i == g.BranchCursor {
			prefix = "▸ "
		}

		// Mark current branch
		branchDisplay := branch
		if branch == currentBranch {
			branchDisplay = g.branchStyle.Render(branch + " ✓")
		} else if i == g.BranchCursor {
			branchDisplay = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#89b4fa")).
				Render(branch)
		}

		b.WriteString(fmt.Sprintf("%s%s\n", prefix, branchDisplay))
	}

	return b.String()
}

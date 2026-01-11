# Changelog

All notable changes to Dotsync will be documented in this file.

## [Unreleased]

### Added
- **Quick Selection Shortcuts**
  - `M` - Select all modified apps/files (local changes to push)
  - `O` - Select all outdated apps/files (dotfiles changes to pull)
  - `r` - Refresh current view while preserving filters
  - `u` - Undo last selection change (restore previous state)

- **Context-Sensitive Help Bar**
  - Help bar now shows different hints based on current panel
  - Shows push/pull options only when items are selected
  - Panel-specific actions (diff in Files, filter in Apps)

- **Enhanced Panel Titles**
  - Apps panel shows: "📁 Applications (3/15)" - selected/total
  - Files panel shows: "📄 Neovim (5/12)" - selected/total

- **Version Display in Header**
  - Shows version number (e.g., v1.1.0) next to title
  - Shows current git branch when in a git repository

- **Rotating Tips on Scan Screen**
  - Shows different helpful tips while scanning
  - Tips rotate every 3 seconds

- **Comprehensive Keyboard Shortcuts Guide**
  - Full help overlay with categorized keybindings
  - Navigation, Sync, Git, Diff/Merge, and General sections
  - Status icons legend with descriptions

- **Sync Progress Screen**
  - Visual progress bar during push/pull operations
  - Shows current file being synced
  - File count progress (e.g., "5 / 12 files")
  - Uses Charmbracelet bubbles/progress component

- **Enhanced Status Notifications**
  - Color-coded status messages (success: green, error: red, warning: amber)
  - Visual notification styles for important feedback
  - Styled buttons for dialogs

- **Improved UI Components**
  - Using bubbles/viewport for smooth scrolling in large lists
  - bubbles/progress for visual sync progress
  - bubbles/spinner for loading states
  - bubbles/textinput for search and path input
  - bubbles/textarea for multi-line commit messages (Ctrl+S to commit)
  - Position indicator in lists (e.g., "15/42") for better navigation
  - Enhanced FullHelp with 6 categorized keybinding groups

### Changed
- Updated all dependencies to latest versions (bubbles v0.21, bubbletea v1.3, lipgloss v1.1)
- Improved scan feedback with directory listing
- Auto-detection now covers 960+ applications
- Enhanced test coverage across all packages (85%+ average)

### Fixed
- Category filter preserved after refresh operation

---

## [1.0.0] - Core Features

### Added
- **Core Features**
  - Two-way sync between local configs and dotfiles repository
  - Automatic backup before overwriting files
  - Selective file sync with checkbox selection
  - Auto-detection of 776 popular applications

- **Diff & Merge**
  - Side-by-side diff viewer with syntax highlighting
  - Hash-based conflict detection (SHA256)
  - Per-hunk merge tool for fine-grained conflict resolution
  - Support for 40+ programming languages via Chroma

- **Git Integration**
  - Built-in git panel for commit, push, pull, fetch
  - Branch display and ahead/behind tracking
  - Stash support

- **User Interface**
  - Beautiful TUI with Bubble Tea framework
  - Keyboard-driven navigation
  - Status icons for sync state (✓ ● ○ ⚡ + ↓ ✗)
  - Help overlay with keybinding reference

- **Supported Applications**
  - AI Tools: Claude Code, GitHub Copilot
  - Shells: Zsh, Bash, Fish, Oh My Zsh, Starship
  - Editors: Neovim, Vim, Emacs, VS Code, Cursor, Zed, Helix
  - Terminals: Kitty, Alacritty, iTerm2, WezTerm, Ghostty, Zellij
  - Git: Git, LazyGit, GitHub CLI
  - Dev Tools: Tmux, Docker, SSH, AWS CLI, direnv, mise, asdf, Nix, NPM, Pip, Cargo, Go
  - Linters/Formatters: ESLint, Prettier, Pylint, EditorConfig
  - CLI: fzf, ripgrep, fd, bat, lsd, eza, zoxide, atuin, yazi, ranger, lf, nnn, wget, curl, jq, yq, sd, jless, fx, Miller, xsv, entr
  - System Monitors: htop, btop, bottom
  - Productivity: Karabiner, Raycast, Yabai, AeroSpace, Hammerspoon, skhd, SketchyBar
  - Kubernetes: K9s, LazyDocker

- **Developer Experience**
  - 390 unit tests (484 test runs) with comprehensive coverage
  - CI/CD with GitHub Actions
  - Cross-platform builds via GoReleaser
  - Makefile for common tasks

### Test Coverage
| Package | Coverage |
|---------|----------|
| `models` | 98.6% |
| `scanner` | 96.4% |
| `ui/components` | 90.2% |
| `sync` | 90.3% |
| `config` | 80.5% |
| `git` | 75.6% |
| `ui` | 73.8% |

### Dependencies
- github.com/charmbracelet/bubbletea - TUI framework
- github.com/charmbracelet/lipgloss - Styling
- github.com/charmbracelet/bubbles - TUI components
- github.com/go-git/go-git/v5 - Pure Go git implementation
- github.com/sergi/go-diff - Diff algorithm
- github.com/alecthomas/chroma/v2 - Syntax highlighting

## [0.1.0] - Initial Release

- First public release
- Basic sync functionality
- TUI interface

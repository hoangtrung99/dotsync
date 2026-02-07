# Dotsync Project Memory

## Project Overview
Dotsync is a TUI (Terminal User Interface) application for managing and syncing dotfiles across machines. Built in Go using Bubble Tea framework.

## Key Features
- Auto-detect 960+ apps and their config files
- Two-way sync (push local to dotfiles repo, pull from repo to local)
- Git integration (commit, push, pull, branch switching)
- Diff viewer with syntax highlighting
- Conflict detection and merge tool
- Brewfile export

## Tech Stack
- **Language**: Go 1.24+
- **TUI**: Bubble Tea + Lip Gloss + Bubbles
- **Git**: go-git (pure Go implementation)
- **Diff**: go-diff
- **Syntax Highlighting**: Chroma

## Project Structure
```
dotsync/
├── main.go              # Main entry point (~60k lines)
├── internal/            # Internal packages
├── configs/             # Default app definitions
├── Makefile             # Build commands
├── go.mod / go.sum      # Dependencies
└── CLAUDE.md            # This file
```

## Build Commands
```bash
make build      # Build optimized binary
make install    # Install to ~/.local/bin
make run        # Build and run
make test       # Run tests
make coverage   # Run tests with coverage
```

## Syncing Claude Code Configuration

### Files to Sync (User-level)
| File/Directory | Purpose | Sync Method |
|----------------|---------|-------------|
| `~/.claude/settings.json` | User preferences | Dotfile manager |
| `~/.claude/CLAUDE.md` | User memory | Dotfile manager |
| `~/.claude/agents/` | Custom subagents | Dotfile manager |
| `~/.claude/skills/` | Custom skills | Dotfile manager |
| `~/.claude/plugins/` | Installed plugins | Auto-reinstalled from settings |

### Files NOT to Sync
- `~/.claude.json` - Contains OAuth tokens and API keys (re-authenticate on each machine)
- `~/.claude/cache/` - Temporary cache
- `~/.claude/projects/` - Project-specific conversations

### Project-level Configs (Commit to Git)
| File/Directory | Purpose |
|----------------|---------|
| `.claude/settings.json` | Project settings |
| `CLAUDE.md` or `.claude/CLAUDE.md` | Project memory |
| `.claude/agents/` | Project subagents |
| `.mcp.json` | MCP server configs |

### Settings Precedence
managed > local > project > user (more specific overrides less specific)

## Workflow for Syncing Claude Code with Dotsync

1. **Push Claude Code configs to dotfiles**:
   - Run `dotsync`
   - Select "Claude Code" from AI Tools category (press `1`)
   - Press `p` to push configs to dotfiles repo

2. **On new machine**:
   - Clone dotfiles repo
   - Run `dotsync`
   - Select Claude Code
   - Press `l` to pull configs
   - Run `claude` and re-authenticate

## Development Notes
- Binary output: `./dotsync` (12MB optimized)
- Test coverage: ~390 tests across all packages
- Supports macOS, Linux (Wayland/X11), WSL

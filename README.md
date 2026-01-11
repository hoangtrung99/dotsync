# Dotsync

A beautiful TUI application for managing and syncing dotfiles across machines.

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-blue.svg)
![Build](https://img.shields.io/github/actions/workflow/status/yourusername/dotsync/ci.yml?branch=main)
![Release](https://img.shields.io/github/v/release/yourusername/dotsync)

## Features

- **Auto-detect apps** - Automatically discovers installed applications and their config files (960+ supported)
- **Two-way sync** - Push local configs to dotfiles repo, or pull from dotfiles to local
- **Diff viewer** - Side-by-side comparison with syntax highlighting (40+ languages)
- **Conflict detection** - Smart hash-based detection when both sides have changes
- **Merge tool** - Per-hunk conflict resolution for fine-grained control
- **Git integration** - Built-in git operations (add, commit, push, pull, fetch, stash, branch switching)
- **Brewfile export** - Export Homebrew packages to a Brewfile for easy machine setup
- **Selective sync** - Choose exactly which files to sync
- **Backup support** - Automatic backup before overwriting files
- **Syntax highlighting** - Beautiful code highlighting using Chroma

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/dotsync.git
cd dotsync

# Build
go build -o dotsync .

# Move to PATH (optional)
mv dotsync ~/.local/bin/
```

## Quick Start

Get up and running in under 5 minutes:

### 1. Install & Run

```bash
# Clone and build
git clone https://github.com/yourusername/dotsync.git
cd dotsync && go build -o dotsync .

# Run
./dotsync
```

### 2. First-Time Setup

On first run, dotsync will ask where to store your dotfiles:

```
┌─ 🔄 Welcome to Dotsync! ─────────────────────┐
│                                               │
│  📁 Choose Dotfiles Location                  │
│                                               │
│  [1] ~/dotfiles (recommended)                 │
│  [2] ~/.dotfiles                              │
│  [3] ~/dotfiles-backup                        │
│                                               │
│  Or enter custom path: ~/my-dotfiles          │
└───────────────────────────────────────────────┘
```

Press `1` for the default `~/dotfiles` location.

### 3. Backup Your Configs (Push)

```
1. Use ↑/↓ to navigate to an app (e.g., Neovim)
2. Press Space to select it
3. Press Tab to switch to the files panel
4. Press 'a' to select all files
5. Press 'p' to PUSH
6. Review the confirmation showing files to push
7. Press Enter to confirm → Files copied to ~/dotfiles/neovim/
```

### 4. Commit & Push to Git

```
1. Press 'g' to open the Git panel
2. Press 'a' to stage all changes
3. Press 'c' to commit (type your message, press Enter)
4. Press 'p' to push to remote
```

### 5. Restore on Another Machine

```bash
# On the new machine:
git clone git@github.com:username/dotfiles.git ~/dotfiles
./dotsync

# Then in dotsync:
# 1. Select apps to restore
# 2. Press 'l' to PULL → Configs restored to your system
```

That's it! Your dotfiles are now synced.

### Quick Reference

| What you want to do | Keys |
|---------------------|------|
| Search apps | `/` then type |
| Select app/file | `Space` |
| Select all modified | `M` |
| Select all outdated | `O` |
| Push to dotfiles | `p` |
| Pull from dotfiles | `l` |
| View file diff | `d` |
| Git operations | `g` |
| Refresh view | `r` |
| Export Brewfile | `b` |
| Help screen | `?` |

```bash
# Show version
./dotsync --version

# Show help
./dotsync --help
```

## Usage

### Quick Reference Card

```
┌─────────────────────────────────────────────────────────────────┐
│  🚀 DOTSYNC QUICK REFERENCE                                     │
├─────────────────────────────────────────────────────────────────┤
│  NAVIGATION           │  SYNC               │  GIT             │
│  ↑/k ↓/j  Move        │  p    Push →        │  g   Git panel   │
│  Tab     Switch panel │  l    Pull ←        │  c   Commit      │
│  Space   Toggle       │  d    View diff     │  ^S  Save commit │
│  /       Search       │  m    Merge         │  p   Push        │
│  1-9     Filter       │  r    Refresh       │  f   Fetch       │
├─────────────────────────────────────────────────────────────────┤
│  SELECTION            │  STATUS ICONS                          │
│  a    Select all      │  ✓ Synced    ● Modified (push)         │
│  D    Deselect all    │  ○ Outdated  ⚡ Conflict                │
│  M    Select modified │  + New       ↓ Missing                 │
│  O    Select outdated │                                        │
│  u    Undo selection  │  ?  Help     q  Quit                   │
└─────────────────────────────────────────────────────────────────┘
```

### Main Screen

The main screen shows two panels:
- **Left panel**: List of detected apps with configs
- **Right panel**: Files for the selected app

The status bar at the bottom shows:
- Current panel indicator (📁 for apps, 📄 for files)
- Status message
- Selected apps/files count
- Modified files count
- Conflict count (if any)

### Keybindings

#### Navigation
| Key | Action |
|-----|--------|
| `/` | Search/filter apps |
| `1-9` | Quick filter by category |
| `0` | Clear category filter |
| `↑/k` | Move up |
| `↓/j` | Move down |
| `PgUp/Ctrl+u` | Page up |
| `PgDn/Ctrl+d` | Page down |
| `Home/g` | Go to first item |
| `End/G` | Go to last item |
| `Tab` | Switch between panels |
| `Space` | Toggle selection |
| `a` | Select all |
| `D` | Deselect all |
| `M` | Select all modified items |
| `O` | Select all outdated items (need pull) |
| `u` | Undo last selection change |

**Category Shortcuts:**
| Key | Category |
|-----|----------|
| `1` | AI Tools |
| `2` | Shells |
| `3` | Editors |
| `4` | Terminals |
| `5` | Git Tools |
| `6` | Dev Tools |
| `7` | CLI Tools |
| `8` | Productivity |
| `9` | Cloud/Infra |

#### Sync Operations
| Key | Action |
|-----|--------|
| `p` | **Push** - Copy local configs to dotfiles |
| `l` | **Pull** - Copy from dotfiles to local |
| `s` | Rescan for apps |
| `r` | Refresh current view |
| `b` | Export Brewfile |

#### Diff & Merge
| Key | Action |
|-----|--------|
| `d` | View diff for selected file |
| `m` | Open merge tool (in diff view) |
| `n` | Next hunk |
| `N` | Previous hunk |
| `1` | Keep local version |
| `2` | Use dotfiles version |
| `h` | Toggle syntax highlighting |
| `Enter` | Save merge (when all resolved) |

#### Git Operations
| Key | Action |
|-----|--------|
| `g` | Open git panel |
| `a` | Stage all changes (in git panel) |
| `c` | Commit (opens multi-line editor) |
| `Ctrl+S` | Submit commit message |
| `p` | Push to remote |
| `f` | Fetch from remote |
| `l` | Pull from remote |
| `s` | Stash changes |
| `S` | Stash pop |
| `b` | Toggle branch mode |
| `Enter` | Checkout selected branch (in branch mode) |
| `r` | Refresh git status |

#### General
| Key | Action |
|-----|--------|
| `?` | Toggle help |
| `Esc` | Go back / Cancel |
| `q` | Quit |

## Workflow

### Push Flow (Local → Dotfiles)
```
┌──────────────┐      Push      ┌──────────────────┐
│ Local Config │  ───────────>  │ ~/dotfiles/{app} │
│ ~/.config/x  │                │ (git repo)       │
└──────────────┘                └──────────────────┘
```

1. Select apps and files you want to sync
2. Press `p` to push
3. Review the confirmation dialog showing files to push
4. Select "Push" to proceed or "Cancel" to abort
5. Files are copied to your dotfiles directory
6. Press `g` to open git panel
7. Press `a` to stage, `c` to commit, `p` to push

### Pull Flow (Dotfiles → Local)
```
┌──────────────────┐    Pull    ┌──────────────┐
│ ~/dotfiles/{app} │  ───────>  │ Local Config │
│ (git repo)       │            │ ~/.config/x  │
└──────────────────┘            └──────────────┘
```

1. Select apps and files to pull
2. Press `l` to pull
3. Review the diff preview
4. Choose: Backup & Pull, Pull Only, or Cancel

### Conflict Resolution
When both local and dotfiles have changes:

1. File shows `⚡` conflict icon
2. Press `d` to view diff
3. Press `m` to enter merge mode
4. For each hunk:
   - Press `1` to keep local
   - Press `2` to use dotfiles
5. Press `Enter` to save merged result

### Brewfile Export
Export all your Homebrew packages to a Brewfile for easy machine setup:

1. Press `b` to export Brewfile
2. File is saved to `~/dotfiles/homebrew/Brewfile`
3. Commit and push to your dotfiles repo
4. On a new machine: `brew bundle install --file=~/dotfiles/homebrew/Brewfile`

The Brewfile includes:
- All installed formulae
- All installed casks
- Custom taps

## Status Icons

| Icon | Meaning |
|------|---------|
| `✓` | Synced - files are identical |
| `●` | Local modified - push to sync |
| `○` | Outdated - pull to update |
| `⚡` | Conflict - both sides changed |
| `+` | New in local only |
| `↓` | New in dotfiles only |
| `✗` | Deleted |

## Configuration

Config file location: `~/.config/dotsync/dotsync.json`

```json
{
  "dotfiles_path": "/Users/username/dotfiles",
  "backup_path": "/Users/username/.dotfiles-backup",
  "apps_config": ""
}
```

### Custom Apps

You can define custom apps in `~/.config/dotsync/apps.yaml`:

```yaml
apps:
  - id: myapp
    name: My Custom App
    paths:
      - ~/.config/myapp
      - ~/.myapprc
```

## Supported Apps

Dotsync auto-detects 960+ popular applications including:

- **AI Tools**: Claude Code, GitHub Copilot, Ollama, LM Studio, Aider, Continue, OpenAI CLI
- **Shells**: Zsh, Bash, Fish, Oh My Zsh, Starship, Nushell, Elvish, Vivid, Carapace, Sheldon
- **Editors**: Neovim, Vim, Emacs, VS Code, Cursor, Zed, Helix, Sublime Text, JetBrains, LazyVim, AstroNvim, LunarVim, Neovide, Lite XL
- **Terminals**: Kitty, Alacritty, iTerm2, WezTerm, Ghostty, Zellij, GNU Screen, Warp
- **Git**: Git, LazyGit, GitHub CLI, GitUI, Tig, Jujutsu, Sapling, Difftastic, Commitizen, Husky, Lefthook
- **Dev Tools**: Tmux, Docker, SSH, AWS CLI, direnv, mise, asdf, Nix, HTTPie, Insomnia, Task, watchexec, SOPS, chezmoi, yadm, GNU Stow, Just, Hyperfine, Textual, Deno, Vale, Vegeta, ghq
- **Package Managers**: NPM, Yarn, pnpm, Bun, Pip, Poetry, uv, Cargo, Go, Maven, Gradle, Pacman, Paru, Yay, Pikaur, Aura, Pamac, Portage, XBPS, Zypper, DNF, APT, APK, Nala, Flatpak, Snap, AppImage, Homebrew
- **Linters/Formatters**: ESLint, Prettier, Pylint, Ruff, EditorConfig, StyLua, rustfmt, Vale
- **CLI**: fzf, ripgrep, fd, bat, lsd, eza, zoxide, atuin, yazi, ranger, lf, nnn, wget, curl, neofetch, fastfetch, thefuck, tldr, mcfly, delta, dust, procs, navi, broot, xplr, glow, mods, topgrade, pet, silicon, slides, VHS, Freeze, Gum, Frogmouth, Posting, Harlequin, Croc, Pueue, Bandwhich, Grex, Tokei, Tealdeer, gping, Ouch, Choose, mdcat, Pastel, Hexyl, sd, jless, jq, yq, fx, Miller, xsv, csvkit, entr, Viddy, gdu, duf, doggo, curlie, xh, Trippy, oha, peco
- **System Monitors**: htop, btop, bottom, Bandwhich, Zenith, Diskonaut
- **Productivity**: Karabiner, Raycast, Yabai, AeroSpace, Hammerspoon, skhd, SketchyBar, Obsidian, Logseq, zk, Espanso, JankyBorders, WTF, Taskwarrior, Timewarrior
- **Cloud/Infra**: Terraform, kubectl, Helm, K9s, LazyDocker, AWS CLI, gcloud, Azure CLI, Podman, Colima, Kind, Minikube
- **Databases**: pgcli, mycli, redis-cli, LazySQL, Harlequin
- **Security**: GnuPG, mitmproxy, age, Bitwarden CLI, 1Password CLI, gopass, passage, Fail2ban, CrowdSec, SSHGuard, auditd, AppArmor, SELinux, Firejail, Bubblewrap
- **Backup**: Kopia, Restic, Borgmatic, rclone, Syncthing
- **Media**: mpv, cmus, ncspot, Mopidy, Spotifyd, playerctl
- **Download**: aria2, youtube-dl, yt-dlp, gallery-dl, Transmission
- **Documents**: Zathura, Sioyek, Calibre
- **Email**: aerc, Himalaya, Mutt, NeoMutt
- **RSS**: Newsboat, Canto
- **Notifications**: Dunst, Mako
- **Window Managers**: i3, Sway, Hyprland, bspwm, Awesome, dwm, Qtile, River, Wayfire, labwc, Hikari, Niri, dwl
- **Bars & Launchers**: Waybar, Polybar, Rofi, Wofi, dmenu
- **Compositors**: Picom, Compton, Cage
- **Screenshot**: Flameshot, maim, scrot
- **Image Viewers**: feh, sxiv, Nitrogen, imv
- **Communication**: WeeChat, Irssi
- **Browsers**: qutebrowser
- **Wayland**: Swaylock, Swayidle, Kanshi
- **Display**: Gammastep, Redshift
- **Social**: toot, tut, tuir, rtv
- **Calendar**: khal, calcurse, Remind, Wyrd
- **Contacts**: abook, khard
- **More Terminals**: Foot, st, urxvt, Termite, Sakura, XFCE Terminal, Tilix, Konsole, GNOME Terminal
- **More Media**: MPD, ncmpcpp, Cava, Streamlink, pipe-viewer
- **Password Managers**: pass, Browserpass, rofi-pass
- **More Launchers**: bemenu, Fuzzel, tofi
- **More Wayland**: wlogout, nwg-bar, nwg-drawer, nwg-launchers
- **Widgets**: Eww, AGS, Conky
- **Panels**: tint2, Lemonbar, dzen2
- **More WMs**: spectrwm, herbstluftwm, Openbox, Fluxbox
- **Login Managers**: Ly, greetd, tuigreet, emptty, LightDM, SDDM, Lemurs
- **Keyboard Remapping**: kmonad, keyd, xremap, Interception Tools, xcape, Kanata
- **Automation**: ydotool, wtype
- **Clipboard Managers**: wl-clipboard, Clipman, cliphist, CopyQ, Greenclip, Parcellite
- **Network**: NetworkManager, ConnMan, iwd, systemd-networkd, systemd-resolved, wpa_supplicant, hostapd, dnsmasq, Unbound, Stubby, dnscrypt-proxy
- **VPN**: WireGuard, OpenVPN, Tailscale, ZeroTier, Nebula, Headscale
- **Firewall**: firewalld, UFW, nftables, iptables
- **Scheduler**: Cron, Anacron, Fcron, at
- **Logging**: Logrotate, rsyslog, syslog-ng, journald, Loki, Promtail, Vector, Fluent Bit, Fluentd
- **Monitoring**: Prometheus, Grafana, Alertmanager, Node Exporter, cAdvisor, Netdata, Glances
- **Web Servers**: Nginx, Apache, Caddy, Traefik, HAProxy
- **Proxy**: Squid, Privoxy
- **Privacy**: Tor, I2P
- **Databases**: PostgreSQL, MySQL, MariaDB, Redis, MongoDB, SQLite, CockroachDB
- **Search**: Elasticsearch, OpenSearch, Meilisearch, Typesense
- **Messaging**: RabbitMQ, Kafka, NATS, Mosquitto
- **Service Discovery**: Consul, etcd
- **Secrets**: Vault
- **Orchestration**: Nomad
- **DevOps**: Packer, Vagrant, Ansible, Puppet, Chef, SaltStack, Pulumi, CDKTF, OpenTofu, Terragrunt, Atlantis
- **GitOps**: ArgoCD, Flux
- **CI/CD**: Jenkins, Drone, Woodpecker, Concourse, Buildkite, CircleCI, Dagger, Earthly
- **Build Systems**: Bazel, Buck2, Pants, Please, sbt, Mill, Leiningen, Boot, Mix, Rebar3, Cabal, Stack, opam, Dune, Nimble
- **Languages**: Zig, Odin, V, Crystal, Julia, R, GNU Octave, Maxima, SageMath, GAP, Coq, Lean, Agda, Idris, Racket, Guile, Chicken Scheme, SBCL, CLISP, ECL, ABCL, CCL
- **Lisp Dialects**: Allegro CL, LispWorks, Clojure, Babashka, Hy, Fennel, Janet, PicoLisp, newLISP
- **Esoteric Languages**: Forth, Gforth, Factor, Red, REBOL, Io, Wren, Gravity, Squirrel, AngelScript, ChaiScript
- **Ruby Tools**: mruby, JRuby, TruffleRuby, rbenv, RVM, chruby
- **Python Tools**: pyenv, Pyright, mypy, Black, isort, Flake8, Bandit, Pytype, pipx, PDM, Hatch
- **Node.js Tools**: NVM, fnm, Volta, n, Biome, oxlint, Rome, SWC, esbuild, Turborepo, Nx, Lerna, Rush, Changesets, Verdaccio
- **Container**: Podman Desktop, Rancher Desktop, Lima, Finch
- **Kubernetes**: Skaffold, Kustomize, Stern, Kubectx
- **API & Testing**: Bruno, Hurl, k6
- **Notes**: Joplin
- **System**: Fontconfig, GTK, Qt
- **Backup**: Timeshift, Vorta
- **Network**: Charles Proxy, Proxyman
- **macOS Productivity**: Alfred, BetterTouchTool, Keyboard Maestro
- **Code Quality**: SonarLint, Semgrep, Trivy
- **Rust**: Rustup, Cargo, rust-analyzer
- **Java**: SDKMAN, jEnv
- **Go**: gopls, golangci-lint
- **Mobile**: Flutter, Android Studio, Xcode, CocoaPods
- **Data Science**: Jupyter, Conda, Mamba
- **Game Dev**: Godot, Unity
- **Design**: Figma, GIMP, Inkscape, ImageMagick
- **Video & Audio**: FFmpeg, OBS Studio, Audacity, VLC
- **Collaboration**: Slack, Discord, Zoom
- **Browser**: Vimium, Surfingkeys
- **IaC**: AWS CDK
- **Testing**: Jest, Pytest
- **Formatters**: Prettier
- **Observability**: Jaeger, OpenTelemetry
- **Secrets**: Vault CLI, 1Password CLI
- **Build**: Make, Taskfile
- **Remote**: Mosh, Eternal Terminal
- **Sync**: Syncthing, Unison
- **Diagrams**: PlantUML, Mermaid, D2
- **Logs**: lnav, multitail
- **Email**: mbsync, msmtp, notmuch
- **Finance**: Ledger, hledger, Beancount
- **Web Frameworks**: Next.js, Nuxt, Vite
- **SSG**: Hugo, Jekyll, Astro
- **Cloud**: DigitalOcean, Linode, Vultr, Hetzner
- **Container Tools**: Skopeo, Buildah
- **Code Analysis**: Ctags, Cscope
- **Spell Check**: Aspell, Hunspell
- **Screenshot**: Flameshot, Shutter
- **Screencast**: Asciinema, Peek
- **PDF**: pdftk, Poppler
- **Archive**: 7-Zip, Atool
- **System Info**: Screenfetch, Pfetch
- **Disk**: Ncdu, Dua
- **Process**: Supervisor, PM2
- **LSP**: TypeScript, Python, Lua
- **Markdown**: Pandoc, Markdownlint
- **Additional AI**: Cursor Rules, Sourcegraph Cody, Tabby
- **macOS Productivity**: Finicky, Rectangle, Hidden Bar
- **X11 Config**: Xresources, Xmodmap, Xinit, Xorg
- **Input Method**: Fcitx, IBus
- **Cloud Auth**: Granted, saml2aws
- **File Managers**: Superfile, Yazi
- **Modern CLI Tools**: Bat, Fd, Procs, Dust, Bottom, Broot, Zoxide
- **Shell History**: Atuin, McFly
- **Cheatsheets**: Navi, Tealdeer
- **Markdown Readers**: Glow, Charm
- **Task Management**: Pueue, Taskwarrior, Timewarrior, Todoman
- **Calendar Sync**: Vdirsyncer
- **News/Social**: Newsboat, Tuir
- **Music Players**: Spotifyd, Spotify TUI, Ncspot, Termusic, Musikcube, Castero
- **Modern Git**: Gitu, Gitoxide
- **Modern Editors**: Lapce, Windsurf, Zed
- **Dev Environments**: DevPod, Devbox, Flox
- **API Clients**: ATAC, Slumber
- **CLI Tools**: Serpl, Television, Erdtree, Amber
- **Container Tools**: Oxker, OrbStack
- **Database TUI**: Gobang, DBLab
- **Security**: RustScan, Feroxbuster
- **Shell History**: Hishtory
- **Notes CLI**: nb, dn
- **System Info**: Macchina, Onefetch
- **Tunneling**: zrok, ngrok, Bore
- **File Servers**: Miniserve, Dufs
- **Rust Tools**: Bacon, Cargo Watch
- **Python Tools**: Rye, Pixi
- **Image Tools**: viu, Chafa
- **Docs**: Zola
- **CLI Utilities**: hwatch, xcp
- **Terminal Enhancements**: Fig, iTerm2 Shell Integration, Ghostty Themes
- **Zsh Plugin Managers**: Zinit, Antidote, Zap
- **AI & LLM Tools**: Ollama, LM Studio, LocalAI, Aider, Continue.dev
- **Modern Terminals**: Warp, Rio, Tabby, Wave
- **Kubernetes Dev**: Kind, K3d, Minikube
- **Container Runtimes**: Colima, OrbStack
- **Database Clients**: DBeaver, TablePlus, usql
- **Encryption Tools**: Age, SOPS
- **Dotfile Managers**: Chezmoi
- **Note Taking**: Obsidian, Logseq, Zettlr
- **API Clients**: Insomnia, HTTPie, Posting
- **Version Managers**: Mise, Proto
- **Python Ecosystem**: UV, Rye, Pixi
- **JavaScript Runtimes**: Bun, Deno
- **AI Editors**: Cursor, Windsurf, Cline
- **AI APIs**: OpenAI, Anthropic, Mistral, Groq, Perplexity, OpenRouter
- **AI Platforms**: Modal, Replicate, Together AI, Fireworks, Anyscale
- **ML Tools**: Hugging Face, Weights & Biases, MLflow, DVC, LangChain, LlamaIndex
- **Modern Git GUIs**: GitButler, Fork, GitKraken, Sourcetree
- **Modern Browsers**: Arc, Zen Browser, Floorp, Vivaldi, Brave
- **Cloud Platforms**: Vercel, Netlify, Railway, Fly.io, Render, SST
- **Database Services**: Supabase, PlanetScale, Turso, Neon, Prisma
- **macOS Menu Bars**: Ice, Bartender, Hidden Bar
- **Productivity Apps**: Linear, Notion, Craft, Anytype
- **Secrets Management**: Infisical, Doppler
- **Cloud Access**: Granted, Leapp
- **Desktop Frameworks**: Tauri, Wails, Encore

## Multi-Machine Workflow

```
Machine A                    GitHub                    Machine B
┌─────────┐                ┌──────────┐              ┌─────────┐
│ Edit    │ ──push──────>  │          │  <──pull──  │         │
│ config  │ ──git push──>  │ dotfiles │  <──git pull │ Apply   │
│         │                │   repo   │              │ config  │
└─────────┘                └──────────┘              └─────────┘
```

1. On Machine A: Edit configs, push to dotfiles, git push
2. On Machine B: git pull, pull configs from dotfiles

## Building from Source

Requirements:
- Go 1.24 or later

```bash
# Get dependencies
go mod download

# Build (standard)
go build -o dotsync .

# Build (optimized, smaller binary)
go build -ldflags="-s -w" -o dotsync .

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

### Using Makefile

```bash
make build      # Build optimized binary
make install    # Install to ~/.local/bin
make run        # Build and run
make test       # Run tests
make coverage   # Run tests with coverage
make clean      # Remove build artifacts
make help       # Show all commands
```

### Test Coverage

| Package | Coverage |
|---------|----------|
| `models` | 98.6% |
| `scanner` | 97.1% |
| `brew` | 91.2% |
| `sync` | 90.3% |
| `ui/components` | 85.2% |
| `config` | 80.5% |
| `ui` | 78.2% |
| `git` | 75.6% |

Total: 390 unit tests (484 test runs including sub-tests)

## Dependencies

### TUI Framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) v1.3+ - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) v1.1+ - Styling
- [Bubbles](https://github.com/charmbracelet/bubbles) v0.21+ - TUI components (spinner, progress, textinput, viewport, table)

### Core Libraries
- [go-git](https://github.com/go-git/go-git) v5.16+ - Pure Go git implementation
- [go-diff](https://github.com/sergi/go-diff) v1.4+ - Diff algorithm library
- [chroma](https://github.com/alecthomas/chroma) v2.22+ - Syntax highlighting (40+ languages)

## Troubleshooting

### Common Issues

**Q: Dotsync doesn't detect my app**
- Check if the app's config files exist in the expected location
- You can add custom apps in `~/.config/dotsync/apps.yaml`

**Q: Git operations fail**
- Ensure your dotfiles directory is a git repository (`git init`)
- Check if you have proper SSH keys or credentials configured
- For HTTPS, ensure you have a credential helper configured

**Q: Diff shows unexpected changes**
- Line endings (CRLF vs LF) may cause differences
- File permissions are not tracked, only content

**Q: Merge conflicts are not detected**
- Make sure to run dotsync after making changes on both sides
- The sync state is stored in `~/.config/dotsync/sync_state.json`

### Debug Mode

Run with verbose output:
```bash
DOTSYNC_DEBUG=1 ./dotsync
```

### Reset Configuration

To reset all settings:
```bash
rm -rf ~/.config/dotsync
```

## FAQ

**Q: Is my data safe?**
A: Yes. Dotsync only copies files, never deletes originals. When pulling, you can choose to backup existing files first.

**Q: Can I use this with an existing dotfiles repo?**
A: Absolutely! Just point dotsync to your existing dotfiles directory during setup.

**Q: Does it support symlinks?**
A: Currently, dotsync copies files. Symlink support is planned for future releases.

**Q: How do I sync to a new machine?**
A:
1. Clone your dotfiles repo: `git clone <your-repo> ~/dotfiles`
2. Run dotsync and point it to `~/dotfiles`
3. Select apps and use Pull (`l`) to restore configs

**Q: Can I exclude certain files?**
A: Yes, simply deselect files you don't want to sync. Selection state is remembered per session.

## Roadmap

- [ ] Symlink mode (link instead of copy)
- [ ] Encryption support for sensitive configs
- [ ] Profile support (work/personal configs)
- [ ] Auto-sync on file changes
- [ ] Cloud sync without git (Dropbox, iCloud, etc.)
- [ ] Plugin system for custom sync logic
- [ ] Remote machine sync via SSH
- [x] Sync progress bar with visual feedback
- [x] Color-coded status notifications
- [x] Undo selection functionality
- [x] Version display in header with git branch
- [x] Rotating tips on scan screen
- [x] Context-sensitive help bar
- [x] Select modified/outdated shortcuts (M/O)
- [x] File count display in panel titles
- [x] Quick action shortcuts (M for modified, r for refresh)
- [x] Push/Pull confirmation dialogs
- [x] Enhanced status bar with file counts
- [x] Page navigation (PgUp/PgDn/Home/End)
- [x] Search and category filtering
- [x] Brewfile export
- [x] Git integration with branch switching
- [x] Merge tool for conflicts
- [x] Diff viewer with syntax highlighting

## Quick Examples

### Backup Neovim config
```bash
# 1. Start dotsync
./dotsync

# 2. Navigate to Neovim (use j/k or arrow keys)
# 3. Press Space to select
# 4. Press Tab to switch to files panel
# 5. Press 'a' to select all files
# 6. Press 'p' to push to dotfiles
```

### Sync to a new machine
```bash
# 1. Clone your dotfiles
git clone git@github.com:username/dotfiles.git ~/dotfiles

# 2. Run dotsync
./dotsync

# 3. Select apps to restore
# 4. Press 'l' to pull from dotfiles
# 5. Choose "Backup & Pull" for safety
```

### Export Homebrew packages
```bash
# In dotsync, press 'b' to export Brewfile
# Then on new machine:
brew bundle install --file=~/dotfiles/homebrew/Brewfile
```

### View and resolve conflicts
```bash
# 1. Files with ⚡ have conflicts
# 2. Press 'd' to view diff
# 3. Press 'm' to merge
# 4. Use '1' for local, '2' for dotfiles
# 5. Press Enter when done
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

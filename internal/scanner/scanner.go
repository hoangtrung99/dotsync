package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"dotsync/internal/models"

	"gopkg.in/yaml.v3"
)

// DebugMode enables debug logging
var DebugMode = false

// debugLog logs a message if debug mode is enabled
func debugLog(format string, args ...interface{}) {
	if DebugMode {
		fmt.Fprintf(os.Stderr, "[SCANNER] "+format+"\n", args...)
	}
}

// Scanner detects installed applications and their config files
type Scanner struct {
	configPath string
	homeDir    string
	brewApps   map[string]bool // Apps installed via Homebrew
	brewMu     sync.RWMutex    // Protects brewApps from concurrent access
}

// New creates a new Scanner
func New(configPath string) *Scanner {
	homeDir, _ := os.UserHomeDir()
	s := &Scanner{
		configPath: configPath,
		homeDir:    homeDir,
		brewApps:   make(map[string]bool),
	}

	// Load brew apps in background - don't block scanner creation
	go s.loadBrewApps()

	return s
}

// loadBrewApps loads list of apps installed via Homebrew
func (s *Scanner) loadBrewApps() {
	start := time.Now()
	debugLog("Loading Homebrew apps...")

	// Get formulae with timeout
	out, err := exec.Command("brew", "list", "--formula", "-1").Output()
	if err == nil {
		for _, app := range strings.Split(string(out), "\n") {
			app = strings.TrimSpace(app)
			if app != "" {
				s.brewMu.Lock()
				s.brewApps[strings.ToLower(app)] = true
				s.brewMu.Unlock()
			}
		}
	}

	// Get casks
	out, err = exec.Command("brew", "list", "--cask", "-1").Output()
	if err == nil {
		for _, app := range strings.Split(string(out), "\n") {
			app = strings.TrimSpace(app)
			if app != "" {
				s.brewMu.Lock()
				s.brewApps[strings.ToLower(app)] = true
				s.brewMu.Unlock()
			}
		}
	}

	s.brewMu.RLock()
	count := len(s.brewApps)
	s.brewMu.RUnlock()
	debugLog("Loaded %d Homebrew apps in %v", count, time.Since(start))
}

// IsBrewInstalled checks if an app is installed via Homebrew
func (s *Scanner) IsBrewInstalled(appName string) bool {
	s.brewMu.RLock()
	defer s.brewMu.RUnlock()
	return s.brewApps[strings.ToLower(appName)]
}

// Scan detects all installed apps and their files using parallel processing
func (s *Scanner) Scan() ([]*models.App, error) {
	start := time.Now()
	debugLog("Starting scan...")

	// Load app definitions
	defs, err := s.loadDefinitions()
	if err != nil {
		// If no config file, use built-in definitions
		defs = s.getBuiltinDefinitions()
	}
	debugLog("Loaded %d app definitions in %v", len(defs), time.Since(start))

	// Use parallel scanning for better performance
	parallelStart := time.Now()
	apps := s.scanAppsParallel(defs)
	debugLog("Parallel scan found %d installed apps in %v", len(apps), time.Since(parallelStart))

	// Also scan for unknown apps in common locations
	unknownStart := time.Now()
	unknownApps := s.scanUnknownApps(apps)
	apps = append(apps, unknownApps...)
	debugLog("Found %d unknown apps in %v", len(unknownApps), time.Since(unknownStart))

	debugLog("Total scan completed in %v", time.Since(start))
	return apps, nil
}

// scanAppsParallel scans apps in parallel using worker pool pattern
func (s *Scanner) scanAppsParallel(defs []models.AppDefinition) []*models.App {
	numWorkers := runtime.NumCPU() * 2 // IO-bound, so use more workers
	if numWorkers > 16 {
		numWorkers = 16 // Cap at 16 workers
	}

	// Channels for work distribution
	jobs := make(chan models.AppDefinition, len(defs))
	results := make(chan *models.App, len(defs))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for def := range jobs {
				if app := s.scanSingleApp(def); app != nil {
					results <- app
				}
			}
		}()
	}

	// Send jobs to workers
	for _, def := range defs {
		jobs <- def
	}
	close(jobs)

	// Wait for all workers to finish and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var apps []*models.App
	for app := range results {
		apps = append(apps, app)
	}

	return apps
}

// scanSingleApp scans a single app definition and returns the app if installed
func (s *Scanner) scanSingleApp(def models.AppDefinition) *models.App {
	app := models.NewApp(def)

	// Check all possible config paths
	for _, configPath := range def.ConfigPaths {
		expandedPath := s.expandPath(configPath)

		if s.pathExists(expandedPath) {
			app.Installed = true

			// Collect files
			files, err := s.collectFiles(expandedPath, def.EncryptedFiles)
			if err == nil {
				app.Files = append(app.Files, files...)
			}
		}
	}

	// Also check Homebrew
	if !app.Installed && s.IsBrewInstalled(def.ID) {
		app.Installed = true
	}

	if app.Installed && len(app.Files) > 0 {
		return app
	}
	return nil
}

// scanUnknownApps scans common config directories for apps not in definitions
func (s *Scanner) scanUnknownApps(knownApps []*models.App) []*models.App {
	var unknown []*models.App

	// Create set of known app IDs
	knownIDs := make(map[string]bool)
	for _, app := range knownApps {
		knownIDs[app.ID] = true
	}

	// Scan ~/.config/
	configDir := filepath.Join(s.homeDir, ".config")
	entries, err := os.ReadDir(configDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			id := strings.ToLower(name)

			// Skip if already known or should be skipped
			if knownIDs[id] || s.shouldSkipDir(name) {
				continue
			}

			// Check if has config files
			dirPath := filepath.Join(configDir, name)
			files, _ := s.collectFiles(dirPath, nil)

			if len(files) > 0 {
				app := &models.App{
					ID:        id,
					Name:      name,
					Category:  "discovered",
					Icon:      "📦",
					Installed: true,
					Files:     files,
				}
				unknown = append(unknown, app)
				knownIDs[id] = true
			}
		}
	}

	return unknown
}

// skipPatterns contains files/dirs to skip during scanning
var skipPatterns = []string{
	".DS_Store", ".git", "node_modules", "__pycache__",
	".cache", "Cache", "CachedData", ".tmp",
	"lock.mdb", "data.mdb",
}

// skipDirs contains directories to skip during discovery
var skipDirs = map[string]bool{
	"configstore": true, "gcloud": true, "yarn": true, "npm": true,
	"cache": true, "caches": true, "logs": true, "tmp": true, "temp": true,
}

// shouldSkipDir returns true if directory should be skipped during discovery
func (s *Scanner) shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return skipDirs[strings.ToLower(name)]
}

// getBuiltinDefinitions returns built-in app definitions
func (s *Scanner) getBuiltinDefinitions() []models.AppDefinition {
	return []models.AppDefinition{
		// AI Tools
		{
			ID:       "claude-code",
			Name:     "Claude Code",
			Category: "ai",
			Icon:     "🤖",
			ConfigPaths: []string{
				// Core settings
				"~/.claude/settings.json",
				"~/.claude.json",
				"~/.claude/.mcp.json",
				// Plugins config
				"~/.claude/plugins/known_marketplaces.json",
				"~/.claude/plugins/installed_plugins.json",
				// Custom extensions (folders)
				"~/.claude/commands",
				"~/.claude/agents",
				"~/.claude/skills",
				"~/.claude/scripts",
			},
			EncryptedFiles: []string{"settings.json"},
		},
		{
			ID:       "github-copilot",
			Name:     "GitHub Copilot",
			Category: "ai",
			Icon:     "🐙",
			ConfigPaths: []string{
				"~/.config/github-copilot",
				"~/Library/Application Support/github-copilot",
			},
		},

		// Terminals
		{
			ID:       "ghostty",
			Name:     "Ghostty",
			Category: "terminal",
			Icon:     "👻",
			ConfigPaths: []string{
				"~/.config/ghostty",
				"~/Library/Application Support/com.mitchellh.ghostty",
			},
		},
		{
			ID:       "kitty",
			Name:     "Kitty",
			Category: "terminal",
			Icon:     "🐱",
			ConfigPaths: []string{
				"~/.config/kitty",
				"~/Library/Preferences/kitty",
			},
		},
		{
			ID:       "alacritty",
			Name:     "Alacritty",
			Category: "terminal",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/alacritty",
				"~/.alacritty.yml",
				"~/.alacritty.toml",
			},
		},
		{
			ID:       "wezterm",
			Name:     "WezTerm",
			Category: "terminal",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.config/wezterm",
				"~/.wezterm.lua",
			},
		},
		{
			ID:       "iterm2",
			Name:     "iTerm2",
			Category: "terminal",
			Icon:     "📟",
			ConfigPaths: []string{
				"~/.config/iterm2",
				"~/Library/Preferences/com.googlecode.iterm2.plist",
				"~/Library/Application Support/iTerm2",
			},
		},

		// Shells
		{
			ID:       "zsh",
			Name:     "Zsh",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.zshrc",
				"~/.zprofile",
				"~/.zshenv",
				"~/.zlogin",
			},
		},
		{
			ID:       "bash",
			Name:     "Bash",
			Category: "shell",
			Icon:     "💲",
			ConfigPaths: []string{
				"~/.bashrc",
				"~/.bash_profile",
				"~/.bash_aliases",
			},
		},
		{
			ID:       "fish",
			Name:     "Fish",
			Category: "shell",
			Icon:     "🐟",
			ConfigPaths: []string{
				"~/.config/fish",
			},
		},
		{
			ID:       "starship",
			Name:     "Starship",
			Category: "shell",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/starship.toml",
				"~/.starship.toml",
			},
		},
		{
			ID:       "oh-my-zsh",
			Name:     "Oh My Zsh",
			Category: "shell",
			Icon:     "✨",
			ConfigPaths: []string{
				"~/.oh-my-zsh/custom",
			},
		},

		// Editors
		{
			ID:       "nvim",
			Name:     "Neovim",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/nvim",
				"~/AppData/Local/nvim", // Windows
			},
		},
		{
			ID:       "vim",
			Name:     "Vim",
			Category: "editor",
			Icon:     "📗",
			ConfigPaths: []string{
				"~/.vimrc",
				"~/.vim",
			},
		},
		{
			ID:       "zed",
			Name:     "Zed",
			Category: "editor",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/zed/settings.json",
				"~/.config/zed/keymap.json",
				"~/.config/zed/themes",
			},
		},
		{
			ID:       "vscode",
			Name:     "VS Code",
			Category: "editor",
			Icon:     "💠",
			ConfigPaths: []string{
				"~/Library/Application Support/Code/User/settings.json",
				"~/Library/Application Support/Code/User/keybindings.json",
				"~/.config/Code/User/settings.json", // Linux
			},
		},
		{
			ID:       "cursor",
			Name:     "Cursor",
			Category: "editor",
			Icon:     "🖱️",
			ConfigPaths: []string{
				"~/Library/Application Support/Cursor/User/settings.json",
				"~/Library/Application Support/Cursor/User/keybindings.json",
			},
		},
		{
			ID:       "emacs",
			Name:     "Emacs",
			Category: "editor",
			Icon:     "🦬",
			ConfigPaths: []string{
				"~/.emacs.d",
				"~/.emacs",
				"~/.doom.d", // Doom Emacs
			},
		},
		{
			ID:       "helix",
			Name:     "Helix",
			Category: "editor",
			Icon:     "🧬",
			ConfigPaths: []string{
				"~/.config/helix",
			},
		},

		// Git
		{
			ID:       "git",
			Name:     "Git",
			Category: "git",
			Icon:     "🔀",
			ConfigPaths: []string{
				"~/.gitconfig",
				"~/.gitignore_global",
				"~/.git-credentials",
			},
			EncryptedFiles: []string{".git-credentials"},
		},
		{
			ID:       "lazygit",
			Name:     "LazyGit",
			Category: "git",
			Icon:     "😴",
			ConfigPaths: []string{
				"~/.config/lazygit",
				"~/Library/Application Support/lazygit",
			},
		},
		{
			ID:       "gh",
			Name:     "GitHub CLI",
			Category: "git",
			Icon:     "🐙",
			ConfigPaths: []string{
				"~/.config/gh",
			},
		},

		// Dev Tools
		{
			ID:       "tmux",
			Name:     "Tmux",
			Category: "dev",
			Icon:     "🪟",
			ConfigPaths: []string{
				"~/.tmux.conf",
				"~/.config/tmux",
				"~/.tmux",
			},
		},
		{
			ID:       "npm",
			Name:     "NPM",
			Category: "dev",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.npmrc",
			},
		},
		{
			ID:       "yarn",
			Name:     "Yarn",
			Category: "dev",
			Icon:     "🧶",
			ConfigPaths: []string{
				"~/.yarnrc",
				"~/.yarnrc.yml",
			},
		},
		{
			ID:       "docker",
			Name:     "Docker",
			Category: "dev",
			Icon:     "🐳",
			ConfigPaths: []string{
				"~/.docker/config.json",
			},
		},
		{
			ID:       "ssh",
			Name:     "SSH",
			Category: "dev",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.ssh/config",
			},
		},
		{
			ID:       "aws",
			Name:     "AWS CLI",
			Category: "dev",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.aws/config",
				"~/.aws/credentials",
			},
			EncryptedFiles: []string{"credentials"},
		},

		// Window Managers / Productivity
		{
			ID:       "raycast",
			Name:     "Raycast",
			Category: "productivity",
			Icon:     "🔦",
			ConfigPaths: []string{
				"~/.config/raycast",
			},
		},
		{
			ID:       "karabiner",
			Name:     "Karabiner",
			Category: "productivity",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/karabiner",
			},
		},
		{
			ID:       "aerospace",
			Name:     "AeroSpace",
			Category: "productivity",
			Icon:     "🪟",
			ConfigPaths: []string{
				"~/.aerospace.toml",
				"~/.config/aerospace",
			},
		},
		{
			ID:       "yabai",
			Name:     "Yabai",
			Category: "productivity",
			Icon:     "🍃",
			ConfigPaths: []string{
				"~/.yabairc",
				"~/.config/yabai",
			},
		},
		{
			ID:       "skhd",
			Name:     "SKHD",
			Category: "productivity",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.skhdrc",
				"~/.config/skhd",
			},
		},
		{
			ID:       "hammerspoon",
			Name:     "Hammerspoon",
			Category: "productivity",
			Icon:     "🔨",
			ConfigPaths: []string{
				"~/.hammerspoon",
			},
		},

		// Misc
		{
			ID:       "btop",
			Name:     "btop",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/btop",
			},
		},
		{
			ID:       "bat",
			Name:     "bat",
			Category: "cli",
			Icon:     "🦇",
			ConfigPaths: []string{
				"~/.config/bat",
			},
		},
		{
			ID:       "lsd",
			Name:     "lsd",
			Category: "cli",
			Icon:     "📂",
			ConfigPaths: []string{
				"~/.config/lsd",
			},
		},
		{
			ID:       "ripgrep",
			Name:     "ripgrep",
			Category: "cli",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/ripgrep",
				"~/.ripgreprc",
			},
		},
		{
			ID:       "fd",
			Name:     "fd",
			Category: "cli",
			Icon:     "🔎",
			ConfigPaths: []string{
				"~/.config/fd",
				"~/.fdignore",
			},
		},
		// Additional popular tools
		{
			ID:       "fzf",
			Name:     "fzf",
			Category: "cli",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.fzf.zsh",
				"~/.fzf.bash",
			},
		},
		{
			ID:       "zoxide",
			Name:     "zoxide",
			Category: "cli",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/zoxide",
			},
		},
		{
			ID:       "atuin",
			Name:     "Atuin",
			Category: "cli",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.config/atuin",
			},
		},
		{
			ID:       "direnv",
			Name:     "direnv",
			Category: "dev",
			Icon:     "📂",
			ConfigPaths: []string{
				"~/.direnvrc",
				"~/.config/direnv",
			},
		},
		{
			ID:       "mise",
			Name:     "mise",
			Category: "dev",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.config/mise",
				"~/.mise.toml",
			},
		},
		{
			ID:       "asdf",
			Name:     "asdf",
			Category: "dev",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.asdfrc",
				"~/.tool-versions",
			},
		},
		{
			ID:       "nix",
			Name:     "Nix",
			Category: "dev",
			Icon:     "❄️",
			ConfigPaths: []string{
				"~/.config/nix",
				"~/.nixpkgs/config.nix",
			},
		},
		{
			ID:       "homebrew",
			Name:     "Homebrew",
			Category: "dev",
			Icon:     "🍺",
			ConfigPaths: []string{
				"~/.Brewfile",
			},
		},
		{
			ID:       "eza",
			Name:     "eza",
			Category: "cli",
			Icon:     "📂",
			ConfigPaths: []string{
				"~/.config/eza",
			},
		},
		{
			ID:       "yazi",
			Name:     "Yazi",
			Category: "cli",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/yazi",
			},
		},
		{
			ID:       "lazydocker",
			Name:     "LazyDocker",
			Category: "dev",
			Icon:     "🐳",
			ConfigPaths: []string{
				"~/.config/lazydocker",
			},
		},
		{
			ID:       "k9s",
			Name:     "K9s",
			Category: "dev",
			Icon:     "☸️",
			ConfigPaths: []string{
				"~/.config/k9s",
			},
		},
		// Additional Tools
		{
			ID:       "bottom",
			Name:     "Bottom",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/bottom",
			},
		},
		{
			ID:       "htop",
			Name:     "Htop",
			Category: "cli",
			Icon:     "📈",
			ConfigPaths: []string{
				"~/.config/htop",
			},
		},
		{
			ID:       "ncspot",
			Name:     "ncspot",
			Category: "cli",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.config/ncspot",
			},
		},
		{
			ID:       "newsboat",
			Name:     "Newsboat",
			Category: "cli",
			Icon:     "📰",
			ConfigPaths: []string{
				"~/.config/newsboat",
				"~/.newsboat",
			},
		},
		{
			ID:       "neomutt",
			Name:     "NeoMutt",
			Category: "cli",
			Icon:     "📧",
			ConfigPaths: []string{
				"~/.config/neomutt",
				"~/.mutt",
			},
		},
		{
			ID:       "ranger",
			Name:     "Ranger",
			Category: "cli",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/ranger",
			},
		},
		{
			ID:       "lf",
			Name:     "lf",
			Category: "cli",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/lf",
			},
		},
		{
			ID:       "nnn",
			Name:     "nnn",
			Category: "cli",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/nnn",
			},
		},
		{
			ID:       "zellij",
			Name:     "Zellij",
			Category: "terminal",
			Icon:     "🪟",
			ConfigPaths: []string{
				"~/.config/zellij",
			},
		},
		{
			ID:       "borders",
			Name:     "JankyBorders",
			Category: "productivity",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/borders",
			},
		},
		{
			ID:       "sketchybar",
			Name:     "SketchyBar",
			Category: "productivity",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/sketchybar",
			},
		},
		// Additional Popular Tools
		{
			ID:       "wget",
			Name:     "Wget",
			Category: "cli",
			Icon:     "📥",
			ConfigPaths: []string{
				"~/.wgetrc",
			},
		},
		{
			ID:       "curl",
			Name:     "cURL",
			Category: "cli",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.curlrc",
			},
		},
		{
			ID:       "pip",
			Name:     "Pip",
			Category: "dev",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/pip",
				"~/.pip",
			},
		},
		{
			ID:       "cargo",
			Name:     "Cargo",
			Category: "dev",
			Icon:     "🦀",
			ConfigPaths: []string{
				"~/.cargo/config.toml",
			},
		},
		{
			ID:       "go",
			Name:     "Go",
			Category: "dev",
			Icon:     "🐹",
			ConfigPaths: []string{
				"~/.config/go",
			},
		},
		{
			ID:       "pylint",
			Name:     "Pylint",
			Category: "dev",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.pylintrc",
				"~/.config/pylint",
			},
		},
		{
			ID:       "prettier",
			Name:     "Prettier",
			Category: "dev",
			Icon:     "✨",
			ConfigPaths: []string{
				"~/.prettierrc",
				"~/.prettierrc.json",
				"~/.prettierrc.yaml",
			},
		},
		{
			ID:       "eslint",
			Name:     "ESLint",
			Category: "dev",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.eslintrc",
				"~/.eslintrc.json",
				"~/.eslintrc.js",
			},
		},
		{
			ID:       "editorconfig",
			Name:     "EditorConfig",
			Category: "dev",
			Icon:     "⚙️",
			ConfigPaths: []string{
				"~/.editorconfig",
			},
		},
		// Additional Modern Tools
		{
			ID:       "pnpm",
			Name:     "pnpm",
			Category: "dev",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.npmrc",
				"~/.config/pnpm",
			},
		},
		{
			ID:       "bun",
			Name:     "Bun",
			Category: "dev",
			Icon:     "🥟",
			ConfigPaths: []string{
				"~/.bunfig.toml",
			},
		},
		{
			ID:       "deno",
			Name:     "Deno",
			Category: "dev",
			Icon:     "🦕",
			ConfigPaths: []string{
				"~/.deno",
			},
		},
		{
			ID:       "poetry",
			Name:     "Poetry",
			Category: "dev",
			Icon:     "📜",
			ConfigPaths: []string{
				"~/.config/pypoetry",
			},
		},
		{
			ID:       "ruff",
			Name:     "Ruff",
			Category: "dev",
			Icon:     "🐕",
			ConfigPaths: []string{
				"~/.config/ruff",
				"~/.ruff.toml",
			},
		},
		{
			ID:       "uv",
			Name:     "uv",
			Category: "dev",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/uv",
			},
		},
		// Cloud & Infrastructure
		{
			ID:       "terraform",
			Name:     "Terraform",
			Category: "dev",
			Icon:     "🏗️",
			ConfigPaths: []string{
				"~/.terraformrc",
				"~/.terraform.d",
			},
		},
		{
			ID:       "kubectl",
			Name:     "kubectl",
			Category: "dev",
			Icon:     "☸️",
			ConfigPaths: []string{
				"~/.kube/config",
			},
		},
		{
			ID:       "helm",
			Name:     "Helm",
			Category: "dev",
			Icon:     "⛵",
			ConfigPaths: []string{
				"~/.config/helm",
			},
		},
		{
			ID:       "gcloud",
			Name:     "Google Cloud CLI",
			Category: "dev",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.config/gcloud",
			},
		},
		{
			ID:       "azure-cli",
			Name:     "Azure CLI",
			Category: "dev",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.azure",
			},
		},
		// Additional Editors/IDEs
		{
			ID:       "sublime",
			Name:     "Sublime Text",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/Library/Application Support/Sublime Text/Packages/User",
			},
		},
		{
			ID:       "jetbrains",
			Name:     "JetBrains IDE",
			Category: "editor",
			Icon:     "🧠",
			ConfigPaths: []string{
				"~/.ideavimrc",
			},
		},
		// Database Tools
		{
			ID:       "pgcli",
			Name:     "pgcli",
			Category: "dev",
			Icon:     "🐘",
			ConfigPaths: []string{
				"~/.config/pgcli",
			},
		},
		{
			ID:       "mycli",
			Name:     "mycli",
			Category: "dev",
			Icon:     "🐬",
			ConfigPaths: []string{
				"~/.myclirc",
			},
		},
		{
			ID:       "redis-cli",
			Name:     "redis-cli",
			Category: "dev",
			Icon:     "🔴",
			ConfigPaths: []string{
				"~/.redisclirc",
			},
		},
		// Knowledge & Notes
		{
			ID:       "obsidian",
			Name:     "Obsidian",
			Category: "productivity",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.obsidian",
			},
		},
		{
			ID:       "logseq",
			Name:     "Logseq",
			Category: "productivity",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.logseq",
			},
		},
		// Additional Dev Tools
		{
			ID:       "act",
			Name:     "Act (GitHub Actions)",
			Category: "dev",
			Icon:     "🎬",
			ConfigPaths: []string{
				"~/.actrc",
			},
		},
		{
			ID:       "just",
			Name:     "Just",
			Category: "dev",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.justfile",
			},
		},
		{
			ID:       "pre-commit",
			Name:     "pre-commit",
			Category: "dev",
			Icon:     "🪝",
			ConfigPaths: []string{
				"~/.pre-commit-config.yaml",
			},
		},
		// Containers
		{
			ID:       "podman",
			Name:     "Podman",
			Category: "dev",
			Icon:     "🦭",
			ConfigPaths: []string{
				"~/.config/containers",
			},
		},
		{
			ID:       "colima",
			Name:     "Colima",
			Category: "dev",
			Icon:     "🐋",
			ConfigPaths: []string{
				"~/.colima",
			},
		},
		// Network & Security
		{
			ID:       "mitmproxy",
			Name:     "mitmproxy",
			Category: "dev",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.mitmproxy",
			},
		},
		{
			ID:       "gnupg",
			Name:     "GnuPG",
			Category: "security",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.gnupg/gpg.conf",
				"~/.gnupg/gpg-agent.conf",
			},
		},
		// Additional Shells
		{
			ID:       "nushell",
			Name:     "Nushell",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.config/nushell",
			},
		},
		{
			ID:       "elvish",
			Name:     "Elvish",
			Category: "shell",
			Icon:     "🧝",
			ConfigPaths: []string{
				"~/.config/elvish",
				"~/.elvish",
			},
		},
		// System Info
		{
			ID:       "neofetch",
			Name:     "Neofetch",
			Category: "cli",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/neofetch",
			},
		},
		{
			ID:       "fastfetch",
			Name:     "Fastfetch",
			Category: "cli",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/fastfetch",
			},
		},
		// CLI Helpers
		{
			ID:       "thefuck",
			Name:     "TheFuck",
			Category: "cli",
			Icon:     "🤬",
			ConfigPaths: []string{
				"~/.config/thefuck",
			},
		},
		{
			ID:       "tldr",
			Name:     "TLDR",
			Category: "cli",
			Icon:     "📖",
			ConfigPaths: []string{
				"~/.config/tldr",
				"~/.tldrc",
			},
		},
		{
			ID:       "mcfly",
			Name:     "McFly",
			Category: "cli",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/mcfly",
			},
		},
		// Development
		{
			ID:       "lazyvim",
			Name:     "LazyVim",
			Category: "editor",
			Icon:     "💤",
			ConfigPaths: []string{
				"~/.config/nvim",
			},
		},
		{
			ID:       "astronvim",
			Name:     "AstroNvim",
			Category: "editor",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/nvim",
			},
		},
		{
			ID:       "lunarvim",
			Name:     "LunarVim",
			Category: "editor",
			Icon:     "🌙",
			ConfigPaths: []string{
				"~/.config/lvim",
			},
		},
		// Container & Orchestration
		{
			ID:       "kind",
			Name:     "Kind",
			Category: "dev",
			Icon:     "🐳",
			ConfigPaths: []string{
				"~/.kind",
			},
		},
		{
			ID:       "minikube",
			Name:     "Minikube",
			Category: "dev",
			Icon:     "☸️",
			ConfigPaths: []string{
				"~/.minikube",
			},
		},
		// More CLI Tools
		{
			ID:       "delta",
			Name:     "Delta",
			Category: "cli",
			Icon:     "Δ",
			ConfigPaths: []string{
				"~/.config/delta",
			},
		},
		{
			ID:       "dust",
			Name:     "Dust",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/dust",
			},
		},
		{
			ID:       "procs",
			Name:     "Procs",
			Category: "cli",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/procs",
			},
		},
		{
			ID:       "gitui",
			Name:     "GitUI",
			Category: "git",
			Icon:     "🔀",
			ConfigPaths: []string{
				"~/.config/gitui",
			},
		},
		// Note-taking & Writing
		{
			ID:       "zk",
			Name:     "zk",
			Category: "productivity",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/zk",
			},
		},
		// Music & Media
		{
			ID:       "cmus",
			Name:     "cmus",
			Category: "cli",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.config/cmus",
				"~/.cmus",
			},
		},
		{
			ID:       "mpv",
			Name:     "mpv",
			Category: "cli",
			Icon:     "🎬",
			ConfigPaths: []string{
				"~/.config/mpv",
			},
		},
		// API Tools
		{
			ID:       "httpie",
			Name:     "HTTPie",
			Category: "dev",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.config/httpie",
			},
		},
		{
			ID:       "insomnia",
			Name:     "Insomnia",
			Category: "dev",
			Icon:     "🌙",
			ConfigPaths: []string{
				"~/.config/Insomnia",
			},
		},
		// Password Managers & Security
		{
			ID:       "bitwarden",
			Name:     "Bitwarden CLI",
			Category: "cli",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.config/Bitwarden CLI",
			},
		},
		{
			ID:       "age",
			Name:     "age",
			Category: "cli",
			Icon:     "🔒",
			ConfigPaths: []string{
				"~/.config/age",
			},
		},
		{
			ID:       "sops",
			Name:     "SOPS",
			Category: "dev",
			Icon:     "🔑",
			ConfigPaths: []string{
				"~/.config/sops",
				"~/.sops.yaml",
			},
		},
		// Task Runners & Build Tools
		{
			ID:       "task",
			Name:     "Task",
			Category: "dev",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/task",
			},
		},
		{
			ID:       "watchexec",
			Name:     "watchexec",
			Category: "dev",
			Icon:     "👁️",
			ConfigPaths: []string{
				"~/.config/watchexec",
			},
		},
		// Terminal Multiplexers
		{
			ID:       "screen",
			Name:     "GNU Screen",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.screenrc",
			},
		},
		// Browsers
		{
			ID:       "qutebrowser",
			Name:     "qutebrowser",
			Category: "cli",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.config/qutebrowser",
			},
		},
		// IRC/Chat
		{
			ID:       "weechat",
			Name:     "WeeChat",
			Category: "cli",
			Icon:     "💬",
			ConfigPaths: []string{
				"~/.config/weechat",
				"~/.weechat",
			},
		},
		{
			ID:       "irssi",
			Name:     "Irssi",
			Category: "cli",
			Icon:     "💬",
			ConfigPaths: []string{
				"~/.irssi",
			},
		},
		// More Development Tools
		{
			ID:       "stylua",
			Name:     "StyLua",
			Category: "dev",
			Icon:     "🌙",
			ConfigPaths: []string{
				"~/.config/stylua",
				"stylua.toml",
			},
		},
		{
			ID:       "taplo",
			Name:     "Taplo",
			Category: "dev",
			Icon:     "📄",
			ConfigPaths: []string{
				"~/.config/taplo",
			},
		},
		{
			ID:       "marksman",
			Name:     "Marksman",
			Category: "dev",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/marksman",
			},
		},
		{
			ID:       "zathura",
			Name:     "Zathura",
			Category: "cli",
			Icon:     "📄",
			ConfigPaths: []string{
				"~/.config/zathura",
			},
		},
		{
			ID:       "aerc",
			Name:     "aerc",
			Category: "cli",
			Icon:     "📧",
			ConfigPaths: []string{
				"~/.config/aerc",
			},
		},
		{
			ID:       "himalaya",
			Name:     "Himalaya",
			Category: "cli",
			Icon:     "📧",
			ConfigPaths: []string{
				"~/.config/himalaya",
			},
		},
		// Additional Productivity Tools
		{
			ID:       "navi",
			Name:     "navi",
			Category: "cli",
			Icon:     "🧭",
			ConfigPaths: []string{
				"~/.config/navi",
			},
		},
		{
			ID:       "broot",
			Name:     "Broot",
			Category: "cli",
			Icon:     "🌳",
			ConfigPaths: []string{
				"~/.config/broot",
			},
		},
		{
			ID:       "xplr",
			Name:     "xplr",
			Category: "cli",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/xplr",
			},
		},
		{
			ID:       "lazysql",
			Name:     "LazySQL",
			Category: "dev",
			Icon:     "🗄️",
			ConfigPaths: []string{
				"~/.config/lazysql",
			},
		},
		{
			ID:       "glow",
			Name:     "Glow",
			Category: "cli",
			Icon:     "✨",
			ConfigPaths: []string{
				"~/.config/glow",
			},
		},
		{
			ID:       "slides",
			Name:     "Slides",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/slides",
			},
		},
		{
			ID:       "vhs",
			Name:     "VHS",
			Category: "cli",
			Icon:     "📹",
			ConfigPaths: []string{
				"~/.config/vhs",
			},
		},
		{
			ID:       "freeze",
			Name:     "Freeze",
			Category: "cli",
			Icon:     "❄️",
			ConfigPaths: []string{
				"~/.config/freeze",
			},
		},
		// More CLI Tools
		{
			ID:       "mods",
			Name:     "Mods",
			Category: "cli",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.config/mods",
			},
		},
		{
			ID:       "soft-serve",
			Name:     "Soft Serve",
			Category: "dev",
			Icon:     "🍦",
			ConfigPaths: []string{
				"~/.config/soft-serve",
			},
		},
		{
			ID:       "wishlist",
			Name:     "Wishlist",
			Category: "cli",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/wishlist",
			},
		},
		{
			ID:       "charm",
			Name:     "Charm",
			Category: "cli",
			Icon:     "✨",
			ConfigPaths: []string{
				"~/.config/charm",
			},
		},
		{
			ID:       "skate",
			Name:     "Skate",
			Category: "cli",
			Icon:     "🛹",
			ConfigPaths: []string{
				"~/.config/skate",
			},
		},
		{
			ID:       "pop",
			Name:     "Pop",
			Category: "cli",
			Icon:     "📧",
			ConfigPaths: []string{
				"~/.config/pop",
			},
		},
		{
			ID:       "chezmoi",
			Name:     "chezmoi",
			Category: "dev",
			Icon:     "🏠",
			ConfigPaths: []string{
				"~/.config/chezmoi",
			},
		},
		{
			ID:       "yadm",
			Name:     "yadm",
			Category: "dev",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/yadm",
			},
		},
		{
			ID:       "stow",
			Name:     "GNU Stow",
			Category: "dev",
			Icon:     "🔗",
			ConfigPaths: []string{
				"~/.stowrc",
			},
		},
		{
			ID:       "topgrade",
			Name:     "Topgrade",
			Category: "cli",
			Icon:     "⬆️",
			ConfigPaths: []string{
				"~/.config/topgrade.toml",
				"~/.config/topgrade",
			},
		},
		// More Editors & IDEs
		{
			ID:       "neovide",
			Name:     "Neovide",
			Category: "editor",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/neovide",
			},
		},
		{
			ID:       "lite-xl",
			Name:     "Lite XL",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/lite-xl",
			},
		},
		// More Terminal Tools
		{
			ID:       "hyperfine",
			Name:     "Hyperfine",
			Category: "cli",
			Icon:     "⏱️",
			ConfigPaths: []string{
				"~/.config/hyperfine",
			},
		},
		{
			ID:       "dircolors",
			Name:     "dircolors",
			Category: "cli",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.dir_colors",
				"~/.dircolors",
			},
		},
		{
			ID:       "inputrc",
			Name:     "Readline",
			Category: "cli",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.inputrc",
			},
		},
		{
			ID:       "hushlogin",
			Name:     "Hushlogin",
			Category: "shell",
			Icon:     "🤫",
			ConfigPaths: []string{
				"~/.hushlogin",
			},
		},
		{
			ID:       "wgetrc",
			Name:     "wgetrc",
			Category: "cli",
			Icon:     "📥",
			ConfigPaths: []string{
				"~/.wgetrc",
			},
		},
		{
			ID:       "curlrc",
			Name:     "curlrc",
			Category: "cli",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.curlrc",
			},
		},
		{
			ID:       "gemrc",
			Name:     "gemrc",
			Category: "dev",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.gemrc",
			},
		},
		// More System Configs
		{
			ID:       "profile",
			Name:     "Shell Profile",
			Category: "shell",
			Icon:     "📄",
			ConfigPaths: []string{
				"~/.profile",
				"~/.bash_profile",
				"~/.zprofile",
			},
		},
		{
			ID:       "xresources",
			Name:     "X Resources",
			Category: "cli",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.Xresources",
				"~/.Xdefaults",
			},
		},
		{
			ID:       "xinitrc",
			Name:     "xinitrc",
			Category: "cli",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.xinitrc",
			},
		},
		// More Development Tools
		{
			ID:       "rustfmt",
			Name:     "rustfmt",
			Category: "dev",
			Icon:     "🦀",
			ConfigPaths: []string{
				"~/.rustfmt.toml",
				"rustfmt.toml",
			},
		},
		{
			ID:       "clippy",
			Name:     "Clippy",
			Category: "dev",
			Icon:     "📎",
			ConfigPaths: []string{
				"~/.config/clippy.toml",
				"clippy.toml",
			},
		},
		{
			ID:       "cargo-config",
			Name:     "Cargo Config",
			Category: "dev",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.cargo/config.toml",
				"~/.cargo/config",
			},
		},
		{
			ID:       "pip-config",
			Name:     "pip Config",
			Category: "dev",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/pip",
				"~/.pip",
			},
		},
		{
			ID:       "maven",
			Name:     "Maven",
			Category: "dev",
			Icon:     "☕",
			ConfigPaths: []string{
				"~/.m2/settings.xml",
			},
		},
		{
			ID:       "gradle",
			Name:     "Gradle",
			Category: "dev",
			Icon:     "🐘",
			ConfigPaths: []string{
				"~/.gradle/gradle.properties",
				"~/.gradle/init.gradle",
			},
		},
		// Additional Tools
		{
			ID:       "espanso",
			Name:     "Espanso",
			Category: "productivity",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/espanso",
			},
		},
		{
			ID:       "jj",
			Name:     "Jujutsu",
			Category: "git",
			Icon:     "🔀",
			ConfigPaths: []string{
				"~/.config/jj",
				"~/.jjconfig.toml",
			},
		},
		{
			ID:       "sapling",
			Name:     "Sapling",
			Category: "git",
			Icon:     "🌱",
			ConfigPaths: []string{
				"~/.config/sapling",
			},
		},
		{
			ID:       "tig",
			Name:     "Tig",
			Category: "git",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.tigrc",
				"~/.config/tig",
			},
		},
		{
			ID:       "difftastic",
			Name:     "Difftastic",
			Category: "git",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/difft",
			},
		},
		{
			ID:       "pet",
			Name:     "Pet",
			Category: "cli",
			Icon:     "🐶",
			ConfigPaths: []string{
				"~/.config/pet",
			},
		},
		{
			ID:       "silicon",
			Name:     "Silicon",
			Category: "cli",
			Icon:     "📸",
			ConfigPaths: []string{
				"~/.config/silicon",
			},
		},
		// More CLI Tools
		{
			ID:       "frogmouth",
			Name:     "Frogmouth",
			Category: "cli",
			Icon:     "🐸",
			ConfigPaths: []string{
				"~/.config/frogmouth",
			},
		},
		{
			ID:       "posting",
			Name:     "Posting",
			Category: "cli",
			Icon:     "📮",
			ConfigPaths: []string{
				"~/.config/posting",
			},
		},
		{
			ID:       "harlequin",
			Name:     "Harlequin",
			Category: "cli",
			Icon:     "🃏",
			ConfigPaths: []string{
				"~/.config/harlequin",
			},
		},
		{
			ID:       "textual",
			Name:     "Textual",
			Category: "dev",
			Icon:     "📟",
			ConfigPaths: []string{
				"~/.config/textual",
			},
		},
		{
			ID:       "gum",
			Name:     "Gum",
			Category: "cli",
			Icon:     "🍬",
			ConfigPaths: []string{
				"~/.config/gum",
			},
		},
		{
			ID:       "croc",
			Name:     "Croc",
			Category: "cli",
			Icon:     "🐊",
			ConfigPaths: []string{
				"~/.config/croc",
			},
		},
		{
			ID:       "warp",
			Name:     "Warp Terminal",
			Category: "terminal",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.warp",
			},
		},
		// More Apps to reach 200
		{
			ID:       "vale",
			Name:     "Vale",
			Category: "dev",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/vale",
				"~/.vale.ini",
			},
		},
		{
			ID:       "commitizen",
			Name:     "Commitizen",
			Category: "git",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.czrc",
				"~/.cz.json",
			},
		},
		{
			ID:       "husky",
			Name:     "Husky",
			Category: "git",
			Icon:     "🐶",
			ConfigPaths: []string{
				"~/.huskyrc",
			},
		},
		{
			ID:       "lefthook",
			Name:     "Lefthook",
			Category: "git",
			Icon:     "🪝",
			ConfigPaths: []string{
				"~/.config/lefthook",
			},
		},
		{
			ID:       "pueue",
			Name:     "Pueue",
			Category: "cli",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/pueue",
			},
		},
		{
			ID:       "bandwhich",
			Name:     "Bandwhich",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/bandwhich",
			},
		},
		{
			ID:       "grex",
			Name:     "Grex",
			Category: "cli",
			Icon:     "🔤",
			ConfigPaths: []string{
				"~/.config/grex",
			},
		},
		{
			ID:       "tokei",
			Name:     "Tokei",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/tokei",
			},
		},
		{
			ID:       "tealdeer",
			Name:     "Tealdeer",
			Category: "cli",
			Icon:     "🦌",
			ConfigPaths: []string{
				"~/.config/tealdeer",
			},
		},
		// More Popular Apps (210+)
		{
			ID:       "wtf",
			Name:     "WTF",
			Category: "productivity",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/wtf",
			},
		},
		{
			ID:       "gping",
			Name:     "gping",
			Category: "cli",
			Icon:     "📡",
			ConfigPaths: []string{
				"~/.config/gping",
			},
		},
		{
			ID:       "ouch",
			Name:     "Ouch",
			Category: "cli",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/ouch",
			},
		},
		{
			ID:       "choose",
			Name:     "Choose",
			Category: "cli",
			Icon:     "✂️",
			ConfigPaths: []string{
				"~/.config/choose",
			},
		},
		{
			ID:       "mdcat",
			Name:     "mdcat",
			Category: "cli",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/mdcat",
			},
		},
		{
			ID:       "vivid",
			Name:     "Vivid",
			Category: "shell",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/vivid",
			},
		},
		{
			ID:       "pastel",
			Name:     "Pastel",
			Category: "cli",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/pastel",
			},
		},
		{
			ID:       "hexyl",
			Name:     "Hexyl",
			Category: "cli",
			Icon:     "🔢",
			ConfigPaths: []string{
				"~/.config/hexyl",
			},
		},
		{
			ID:       "diskonaut",
			Name:     "Diskonaut",
			Category: "system",
			Icon:     "💾",
			ConfigPaths: []string{
				"~/.config/diskonaut",
			},
		},
		{
			ID:       "zenith",
			Name:     "Zenith",
			Category: "system",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/zenith",
			},
		},
		// More Popular CLI Tools (220+)
		{
			ID:       "sd",
			Name:     "sd",
			Category: "cli",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/sd",
			},
		},
		{
			ID:       "jless",
			Name:     "jless",
			Category: "cli",
			Icon:     "📄",
			ConfigPaths: []string{
				"~/.config/jless",
			},
		},
		{
			ID:       "jq",
			Name:     "jq",
			Category: "cli",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.jq",
			},
		},
		{
			ID:       "yq",
			Name:     "yq",
			Category: "cli",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.config/yq",
			},
		},
		{
			ID:       "fx",
			Name:     "fx",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/fx",
			},
		},
		{
			ID:       "miller",
			Name:     "Miller",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.mlrrc",
			},
		},
		{
			ID:       "xsv",
			Name:     "xsv",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/xsv",
			},
		},
		{
			ID:       "csvkit",
			Name:     "csvkit",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.csvkitrc",
			},
		},
		{
			ID:       "entr",
			Name:     "entr",
			Category: "dev",
			Icon:     "👁️",
			ConfigPaths: []string{
				"~/.config/entr",
			},
		},
		// More CLI Tools (230+)
		{
			ID:       "viddy",
			Name:     "Viddy",
			Category: "cli",
			Icon:     "👀",
			ConfigPaths: []string{
				"~/.config/viddy",
				"~/.viddy.toml",
			},
		},
		{
			ID:       "gdu",
			Name:     "gdu",
			Category: "cli",
			Icon:     "💾",
			ConfigPaths: []string{
				"~/.config/gdu",
				"~/.gdu.yaml",
			},
		},
		{
			ID:       "duf",
			Name:     "duf",
			Category: "cli",
			Icon:     "💿",
			ConfigPaths: []string{
				"~/.config/duf",
			},
		},
		{
			ID:       "doggo",
			Name:     "doggo",
			Category: "cli",
			Icon:     "🐕",
			ConfigPaths: []string{
				"~/.config/doggo",
				"~/.doggo",
			},
		},
		{
			ID:       "curlie",
			Name:     "curlie",
			Category: "cli",
			Icon:     "🌀",
			ConfigPaths: []string{
				"~/.config/curlie",
				"~/.curlierc",
			},
		},
		{
			ID:       "xh",
			Name:     "xh",
			Category: "cli",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.config/xh",
			},
		},
		{
			ID:       "trippy",
			Name:     "Trippy",
			Category: "cli",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/trippy",
				"~/.trippy.toml",
			},
		},
		{
			ID:       "oha",
			Name:     "oha",
			Category: "cli",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/oha",
			},
		},
		{
			ID:       "vegeta",
			Name:     "Vegeta",
			Category: "dev",
			Icon:     "🥬",
			ConfigPaths: []string{
				"~/.config/vegeta",
			},
		},
		{
			ID:       "ghq",
			Name:     "ghq",
			Category: "dev",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/ghq",
				"~/.gitconfig",
			},
		},
		{
			ID:       "peco",
			Name:     "peco",
			Category: "cli",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/peco",
				"~/.peco",
			},
		},
		// More apps (240+)
		{
			ID:       "1password-cli",
			Name:     "1Password CLI",
			Category: "security",
			Icon:     "🔒",
			ConfigPaths: []string{
				"~/.config/op",
			},
		},
		{
			ID:       "gopass",
			Name:     "gopass",
			Category: "security",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.config/gopass",
			},
		},
		{
			ID:       "passage",
			Name:     "passage",
			Category: "security",
			Icon:     "🔑",
			ConfigPaths: []string{
				"~/.passage",
			},
		},
		{
			ID:       "kopia",
			Name:     "Kopia",
			Category: "backup",
			Icon:     "💾",
			ConfigPaths: []string{
				"~/.config/kopia",
			},
		},
		{
			ID:       "restic",
			Name:     "Restic",
			Category: "backup",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/restic",
			},
		},
		{
			ID:       "borgmatic",
			Name:     "Borgmatic",
			Category: "backup",
			Icon:     "🗄️",
			ConfigPaths: []string{
				"~/.config/borgmatic",
			},
		},
		{
			ID:       "rclone",
			Name:     "rclone",
			Category: "backup",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.config/rclone",
			},
		},
		{
			ID:       "syncthing",
			Name:     "Syncthing",
			Category: "backup",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/syncthing",
			},
		},
		{
			ID:       "timewarrior",
			Name:     "Timewarrior",
			Category: "productivity",
			Icon:     "⏱️",
			ConfigPaths: []string{
				"~/.config/timewarrior",
				"~/.timewarrior",
			},
		},
		{
			ID:       "taskwarrior",
			Name:     "Taskwarrior",
			Category: "productivity",
			Icon:     "✅",
			ConfigPaths: []string{
				"~/.config/task",
				"~/.taskrc",
			},
		},
		{
			ID:       "canto-ng",
			Name:     "Canto",
			Category: "rss",
			Icon:     "📡",
			ConfigPaths: []string{
				"~/.config/canto-ng",
			},
		},
		{
			ID:       "mopidy",
			Name:     "Mopidy",
			Category: "media",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.config/mopidy",
			},
		},
		{
			ID:       "spotifyd",
			Name:     "Spotifyd",
			Category: "media",
			Icon:     "🎧",
			ConfigPaths: []string{
				"~/.config/spotifyd",
			},
		},
		{
			ID:       "playerctl",
			Name:     "playerctl",
			Category: "media",
			Icon:     "🎮",
			ConfigPaths: []string{
				"~/.config/playerctl",
			},
		},
		// More apps (260+)
		{
			ID:       "aria2",
			Name:     "aria2",
			Category: "download",
			Icon:     "⬇️",
			ConfigPaths: []string{
				"~/.config/aria2",
				"~/.aria2",
			},
		},
		{
			ID:       "youtube-dl",
			Name:     "youtube-dl",
			Category: "download",
			Icon:     "📺",
			ConfigPaths: []string{
				"~/.config/youtube-dl",
			},
		},
		{
			ID:       "yt-dlp",
			Name:     "yt-dlp",
			Category: "download",
			Icon:     "🎬",
			ConfigPaths: []string{
				"~/.config/yt-dlp",
			},
		},
		{
			ID:       "gallery-dl",
			Name:     "gallery-dl",
			Category: "download",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/gallery-dl",
			},
		},
		{
			ID:       "transmission",
			Name:     "Transmission",
			Category: "download",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/transmission",
				"~/.config/transmission-daemon",
			},
		},
		{
			ID:       "calibre",
			Name:     "Calibre",
			Category: "ebook",
			Icon:     "📚",
			ConfigPaths: []string{
				"~/.config/calibre",
			},
		},
		{
			ID:       "sioyek",
			Name:     "Sioyek",
			Category: "document",
			Icon:     "📑",
			ConfigPaths: []string{
				"~/.config/sioyek",
			},
		},
		{
			ID:       "dunst",
			Name:     "Dunst",
			Category: "notification",
			Icon:     "🔔",
			ConfigPaths: []string{
				"~/.config/dunst",
			},
		},
		{
			ID:       "mako",
			Name:     "Mako",
			Category: "notification",
			Icon:     "📢",
			ConfigPaths: []string{
				"~/.config/mako",
			},
		},
		// Window Managers & Compositors (270+)
		{
			ID:       "i3",
			Name:     "i3",
			Category: "wm",
			Icon:     "🪟",
			ConfigPaths: []string{
				"~/.config/i3",
				"~/.i3",
			},
		},
		{
			ID:       "sway",
			Name:     "Sway",
			Category: "wm",
			Icon:     "🌊",
			ConfigPaths: []string{
				"~/.config/sway",
			},
		},
		{
			ID:       "hyprland",
			Name:     "Hyprland",
			Category: "wm",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.config/hypr",
			},
		},
		{
			ID:       "bspwm",
			Name:     "bspwm",
			Category: "wm",
			Icon:     "🌳",
			ConfigPaths: []string{
				"~/.config/bspwm",
			},
		},
		{
			ID:       "awesome",
			Name:     "Awesome WM",
			Category: "wm",
			Icon:     "⭐",
			ConfigPaths: []string{
				"~/.config/awesome",
			},
		},
		{
			ID:       "dwm",
			Name:     "dwm",
			Category: "wm",
			Icon:     "🔷",
			ConfigPaths: []string{
				"~/.dwm",
			},
		},
		{
			ID:       "qtile",
			Name:     "Qtile",
			Category: "wm",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/qtile",
			},
		},
		{
			ID:       "waybar",
			Name:     "Waybar",
			Category: "bar",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/waybar",
			},
		},
		{
			ID:       "polybar",
			Name:     "Polybar",
			Category: "bar",
			Icon:     "📈",
			ConfigPaths: []string{
				"~/.config/polybar",
			},
		},
		{
			ID:       "rofi",
			Name:     "Rofi",
			Category: "launcher",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/rofi",
			},
		},
		// More launchers & utilities (280+)
		{
			ID:       "wofi",
			Name:     "Wofi",
			Category: "launcher",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/wofi",
			},
		},
		{
			ID:       "dmenu",
			Name:     "dmenu",
			Category: "launcher",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.dmenu",
			},
		},
		{
			ID:       "picom",
			Name:     "Picom",
			Category: "compositor",
			Icon:     "✨",
			ConfigPaths: []string{
				"~/.config/picom",
				"~/.config/picom.conf",
			},
		},
		{
			ID:       "compton",
			Name:     "Compton",
			Category: "compositor",
			Icon:     "🌟",
			ConfigPaths: []string{
				"~/.config/compton",
				"~/.compton.conf",
			},
		},
		{
			ID:       "flameshot",
			Name:     "Flameshot",
			Category: "screenshot",
			Icon:     "📸",
			ConfigPaths: []string{
				"~/.config/flameshot",
			},
		},
		{
			ID:       "maim",
			Name:     "maim",
			Category: "screenshot",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/maim",
			},
		},
		{
			ID:       "scrot",
			Name:     "scrot",
			Category: "screenshot",
			Icon:     "📷",
			ConfigPaths: []string{
				"~/.scrotrc",
			},
		},
		{
			ID:       "nitrogen",
			Name:     "Nitrogen",
			Category: "wallpaper",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/nitrogen",
			},
		},
		{
			ID:       "feh",
			Name:     "feh",
			Category: "image",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/feh",
				"~/.fehbg",
			},
		},
		{
			ID:       "sxiv",
			Name:     "sxiv",
			Category: "image",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/sxiv",
			},
		},
		// Additional Apps (265+)
		{
			ID:       "imv",
			Name:     "imv",
			Category: "image",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/imv",
			},
		},
		{
			ID:       "swaylock",
			Name:     "Swaylock",
			Category: "wayland",
			Icon:     "🔒",
			ConfigPaths: []string{
				"~/.config/swaylock",
			},
		},
		{
			ID:       "swayidle",
			Name:     "Swayidle",
			Category: "wayland",
			Icon:     "💤",
			ConfigPaths: []string{
				"~/.config/swayidle",
			},
		},
		{
			ID:       "kanshi",
			Name:     "Kanshi",
			Category: "wayland",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/kanshi",
			},
		},
		{
			ID:       "gammastep",
			Name:     "Gammastep",
			Category: "display",
			Icon:     "🌡️",
			ConfigPaths: []string{
				"~/.config/gammastep",
			},
		},
		{
			ID:       "redshift",
			Name:     "Redshift",
			Category: "display",
			Icon:     "🌅",
			ConfigPaths: []string{
				"~/.config/redshift",
				"~/.config/redshift.conf",
			},
		},
		{
			ID:       "foot",
			Name:     "Foot",
			Category: "terminal",
			Icon:     "🦶",
			ConfigPaths: []string{
				"~/.config/foot",
			},
		},
		{
			ID:       "st",
			Name:     "st",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/st",
			},
		},
		{
			ID:       "urxvt",
			Name:     "urxvt",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.Xresources",
				"~/.Xdefaults",
			},
		},
		{
			ID:       "termite",
			Name:     "Termite",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/termite",
			},
		},
		{
			ID:       "sakura",
			Name:     "Sakura",
			Category: "terminal",
			Icon:     "🌸",
			ConfigPaths: []string{
				"~/.config/sakura",
			},
		},
		{
			ID:       "xfce4-terminal",
			Name:     "XFCE Terminal",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/xfce4/terminal",
			},
		},
		{
			ID:       "tilix",
			Name:     "Tilix",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/tilix",
			},
		},
		{
			ID:       "konsole",
			Name:     "Konsole",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/konsolerc",
				"~/.local/share/konsole",
			},
		},
		{
			ID:       "gnome-terminal",
			Name:     "GNOME Terminal",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/gnome-terminal",
			},
		},
		// More apps (280+)
		{
			ID:       "mpd",
			Name:     "MPD",
			Category: "media",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.config/mpd",
				"~/.mpdconf",
			},
		},
		{
			ID:       "ncmpcpp",
			Name:     "ncmpcpp",
			Category: "media",
			Icon:     "🎶",
			ConfigPaths: []string{
				"~/.config/ncmpcpp",
				"~/.ncmpcpp",
			},
		},
		{
			ID:       "mpc",
			Name:     "mpc",
			Category: "media",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.config/mpc",
			},
		},
		{
			ID:       "cava",
			Name:     "Cava",
			Category: "media",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/cava",
			},
		},
		{
			ID:       "vis",
			Name:     "vis",
			Category: "media",
			Icon:     "📈",
			ConfigPaths: []string{
				"~/.config/vis",
			},
		},
		{
			ID:       "pipe-viewer",
			Name:     "pipe-viewer",
			Category: "media",
			Icon:     "📺",
			ConfigPaths: []string{
				"~/.config/pipe-viewer",
			},
		},
		{
			ID:       "streamlink",
			Name:     "Streamlink",
			Category: "media",
			Icon:     "📡",
			ConfigPaths: []string{
				"~/.config/streamlink",
			},
		},
		{
			ID:       "toot",
			Name:     "toot",
			Category: "social",
			Icon:     "🐘",
			ConfigPaths: []string{
				"~/.config/toot",
			},
		},
		{
			ID:       "tut",
			Name:     "tut",
			Category: "social",
			Icon:     "🐘",
			ConfigPaths: []string{
				"~/.config/tut",
			},
		},
		{
			ID:       "tuir",
			Name:     "tuir",
			Category: "social",
			Icon:     "📰",
			ConfigPaths: []string{
				"~/.config/tuir",
			},
		},
		{
			ID:       "rtv",
			Name:     "rtv",
			Category: "social",
			Icon:     "📰",
			ConfigPaths: []string{
				"~/.config/rtv",
			},
		},
		{
			ID:       "neomustrr",
			Name:     "Neomutt",
			Category: "email",
			Icon:     "📧",
			ConfigPaths: []string{
				"~/.config/neomutt",
				"~/.neomuttrc",
			},
		},
		{
			ID:       "abook",
			Name:     "abook",
			Category: "contacts",
			Icon:     "📒",
			ConfigPaths: []string{
				"~/.abook",
			},
		},
		{
			ID:       "khal",
			Name:     "khal",
			Category: "calendar",
			Icon:     "📅",
			ConfigPaths: []string{
				"~/.config/khal",
			},
		},
		{
			ID:       "khard",
			Name:     "khard",
			Category: "contacts",
			Icon:     "📇",
			ConfigPaths: []string{
				"~/.config/khard",
			},
		},
		{
			ID:       "vdirsyncer",
			Name:     "vdirsyncer",
			Category: "sync",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/vdirsyncer",
			},
		},
		{
			ID:       "todoman",
			Name:     "todoman",
			Category: "productivity",
			Icon:     "✅",
			ConfigPaths: []string{
				"~/.config/todoman",
			},
		},
		{
			ID:       "calcurse",
			Name:     "calcurse",
			Category: "calendar",
			Icon:     "📆",
			ConfigPaths: []string{
				"~/.config/calcurse",
				"~/.calcurse",
			},
		},
		{
			ID:       "remind",
			Name:     "Remind",
			Category: "calendar",
			Icon:     "🔔",
			ConfigPaths: []string{
				"~/.reminders",
				"~/.remind",
			},
		},
		{
			ID:       "wyrd",
			Name:     "Wyrd",
			Category: "calendar",
			Icon:     "📅",
			ConfigPaths: []string{
				"~/.wyrdrc",
			},
		},
		// More apps (300+)
		{
			ID:       "pass",
			Name:     "pass",
			Category: "security",
			Icon:     "🔑",
			ConfigPaths: []string{
				"~/.password-store",
			},
		},
		{
			ID:       "browserpass",
			Name:     "Browserpass",
			Category: "security",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.config/browserpass",
			},
		},
		{
			ID:       "rofi-pass",
			Name:     "rofi-pass",
			Category: "security",
			Icon:     "🔑",
			ConfigPaths: []string{
				"~/.config/rofi-pass",
			},
		},
		{
			ID:       "bemenu",
			Name:     "bemenu",
			Category: "launcher",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/bemenu",
			},
		},
		{
			ID:       "fuzzel",
			Name:     "Fuzzel",
			Category: "launcher",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/fuzzel",
			},
		},
		{
			ID:       "tofi",
			Name:     "tofi",
			Category: "launcher",
			Icon:     "🔎",
			ConfigPaths: []string{
				"~/.config/tofi",
			},
		},
		{
			ID:       "wlogout",
			Name:     "wlogout",
			Category: "wayland",
			Icon:     "🚪",
			ConfigPaths: []string{
				"~/.config/wlogout",
			},
		},
		{
			ID:       "nwg-bar",
			Name:     "nwg-bar",
			Category: "wayland",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/nwg-bar",
			},
		},
		{
			ID:       "nwg-drawer",
			Name:     "nwg-drawer",
			Category: "wayland",
			Icon:     "📂",
			ConfigPaths: []string{
				"~/.config/nwg-drawer",
			},
		},
		{
			ID:       "nwg-launchers",
			Name:     "nwg-launchers",
			Category: "wayland",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/nwg-launchers",
			},
		},
		{
			ID:       "eww",
			Name:     "Eww",
			Category: "widgets",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/eww",
			},
		},
		{
			ID:       "ags",
			Name:     "AGS",
			Category: "widgets",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/ags",
			},
		},
		{
			ID:       "conky",
			Name:     "Conky",
			Category: "widgets",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/conky",
				"~/.conkyrc",
			},
		},
		{
			ID:       "tint2",
			Name:     "tint2",
			Category: "panel",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/tint2",
			},
		},
		{
			ID:       "lemonbar",
			Name:     "Lemonbar",
			Category: "panel",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/lemonbar",
			},
		},
		{
			ID:       "dzen2",
			Name:     "dzen2",
			Category: "panel",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/dzen2",
			},
		},
		{
			ID:       "spectrwm",
			Name:     "spectrwm",
			Category: "wm",
			Icon:     "🪟",
			ConfigPaths: []string{
				"~/.config/spectrwm",
				"~/.spectrwm.conf",
			},
		},
		{
			ID:       "herbstluftwm",
			Name:     "herbstluftwm",
			Category: "wm",
			Icon:     "🌿",
			ConfigPaths: []string{
				"~/.config/herbstluftwm",
			},
		},
		{
			ID:       "openbox",
			Name:     "Openbox",
			Category: "wm",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/openbox",
			},
		},
		{
			ID:       "fluxbox",
			Name:     "Fluxbox",
			Category: "wm",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.fluxbox",
			},
		},
		// More apps (350+)
		{
			ID:       "ly",
			Name:     "Ly",
			Category: "login",
			Icon:     "🔐",
			ConfigPaths: []string{
				"/etc/ly",
			},
		},
		{
			ID:       "greetd",
			Name:     "greetd",
			Category: "login",
			Icon:     "🔐",
			ConfigPaths: []string{
				"/etc/greetd",
			},
		},
		{
			ID:       "tuigreet",
			Name:     "tuigreet",
			Category: "login",
			Icon:     "🔐",
			ConfigPaths: []string{
				"/etc/greetd",
			},
		},
		{
			ID:       "emptty",
			Name:     "emptty",
			Category: "login",
			Icon:     "🔐",
			ConfigPaths: []string{
				"/etc/emptty",
			},
		},
		{
			ID:       "lightdm",
			Name:     "LightDM",
			Category: "login",
			Icon:     "🔐",
			ConfigPaths: []string{
				"/etc/lightdm",
				"~/.config/lightdm",
			},
		},
		{
			ID:       "sddm",
			Name:     "SDDM",
			Category: "login",
			Icon:     "🔐",
			ConfigPaths: []string{
				"/etc/sddm.conf",
				"/etc/sddm.conf.d",
			},
		},
		{
			ID:       "lemurs",
			Name:     "Lemurs",
			Category: "login",
			Icon:     "🔐",
			ConfigPaths: []string{
				"/etc/lemurs",
			},
		},
		{
			ID:       "cage",
			Name:     "Cage",
			Category: "wayland",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/cage",
			},
		},
		{
			ID:       "river",
			Name:     "River",
			Category: "wm",
			Icon:     "🌊",
			ConfigPaths: []string{
				"~/.config/river",
			},
		},
		{
			ID:       "wayfire",
			Name:     "Wayfire",
			Category: "wm",
			Icon:     "🔥",
			ConfigPaths: []string{
				"~/.config/wayfire.ini",
			},
		},
		{
			ID:       "labwc",
			Name:     "labwc",
			Category: "wm",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/labwc",
			},
		},
		{
			ID:       "hikari",
			Name:     "Hikari",
			Category: "wm",
			Icon:     "☀️",
			ConfigPaths: []string{
				"~/.config/hikari",
			},
		},
		{
			ID:       "niri",
			Name:     "Niri",
			Category: "wm",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/niri",
			},
		},
		{
			ID:       "dwl",
			Name:     "dwl",
			Category: "wm",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/dwl",
			},
		},
		{
			ID:       "kmonad",
			Name:     "kmonad",
			Category: "keyboard",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/kmonad",
			},
		},
		{
			ID:       "keyd",
			Name:     "keyd",
			Category: "keyboard",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"/etc/keyd",
			},
		},
		{
			ID:       "xremap",
			Name:     "xremap",
			Category: "keyboard",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/xremap",
			},
		},
		{
			ID:       "interception-tools",
			Name:     "Interception Tools",
			Category: "keyboard",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"/etc/interception",
			},
		},
		{
			ID:       "xcape",
			Name:     "xcape",
			Category: "keyboard",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/xcape",
			},
		},
		{
			ID:       "kanata",
			Name:     "Kanata",
			Category: "keyboard",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/kanata",
			},
		},
		{
			ID:       "ydotool",
			Name:     "ydotool",
			Category: "automation",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.config/ydotool",
			},
		},
		{
			ID:       "wtype",
			Name:     "wtype",
			Category: "automation",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.config/wtype",
			},
		},
		{
			ID:       "wl-clipboard",
			Name:     "wl-clipboard",
			Category: "clipboard",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/wl-clipboard",
			},
		},
		{
			ID:       "clipman",
			Name:     "Clipman",
			Category: "clipboard",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/clipman",
			},
		},
		{
			ID:       "cliphist",
			Name:     "cliphist",
			Category: "clipboard",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/cliphist",
			},
		},
		{
			ID:       "copyq",
			Name:     "CopyQ",
			Category: "clipboard",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/copyq",
			},
		},
		{
			ID:       "greenclip",
			Name:     "Greenclip",
			Category: "clipboard",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/greenclip.toml",
			},
		},
		{
			ID:       "parcellite",
			Name:     "Parcellite",
			Category: "clipboard",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/parcellite",
			},
		},
		{
			ID:       "networkmanager",
			Name:     "NetworkManager",
			Category: "network",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/NetworkManager",
			},
		},
		{
			ID:       "connman",
			Name:     "ConnMan",
			Category: "network",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/connman",
			},
		},
		{
			ID:       "iwd",
			Name:     "iwd",
			Category: "network",
			Icon:     "📶",
			ConfigPaths: []string{
				"/etc/iwd",
			},
		},
		{
			ID:       "systemd-networkd",
			Name:     "systemd-networkd",
			Category: "network",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/systemd/network",
			},
		},
		{
			ID:       "systemd-resolved",
			Name:     "systemd-resolved",
			Category: "network",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/systemd/resolved.conf",
			},
		},
		// More apps (400+)
		{
			ID:       "wpa_supplicant",
			Name:     "wpa_supplicant",
			Category: "network",
			Icon:     "📶",
			ConfigPaths: []string{
				"/etc/wpa_supplicant",
			},
		},
		{
			ID:       "hostapd",
			Name:     "hostapd",
			Category: "network",
			Icon:     "📶",
			ConfigPaths: []string{
				"/etc/hostapd",
			},
		},
		{
			ID:       "dnsmasq",
			Name:     "dnsmasq",
			Category: "network",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/dnsmasq.conf",
				"/etc/dnsmasq.d",
			},
		},
		{
			ID:       "unbound",
			Name:     "Unbound",
			Category: "network",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/unbound",
			},
		},
		{
			ID:       "stubby",
			Name:     "Stubby",
			Category: "network",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/stubby",
			},
		},
		{
			ID:       "dnscrypt-proxy",
			Name:     "dnscrypt-proxy",
			Category: "network",
			Icon:     "🔒",
			ConfigPaths: []string{
				"/etc/dnscrypt-proxy",
				"~/.config/dnscrypt-proxy",
			},
		},
		{
			ID:       "wireguard",
			Name:     "WireGuard",
			Category: "vpn",
			Icon:     "🔒",
			ConfigPaths: []string{
				"/etc/wireguard",
			},
		},
		{
			ID:       "openvpn",
			Name:     "OpenVPN",
			Category: "vpn",
			Icon:     "🔒",
			ConfigPaths: []string{
				"/etc/openvpn",
			},
		},
		{
			ID:       "tailscale",
			Name:     "Tailscale",
			Category: "vpn",
			Icon:     "🔗",
			ConfigPaths: []string{
				"/etc/tailscale",
				"~/.config/tailscale",
			},
		},
		{
			ID:       "zerotier",
			Name:     "ZeroTier",
			Category: "vpn",
			Icon:     "🔗",
			ConfigPaths: []string{
				"/var/lib/zerotier-one",
			},
		},
		{
			ID:       "nebula",
			Name:     "Nebula",
			Category: "vpn",
			Icon:     "🌌",
			ConfigPaths: []string{
				"/etc/nebula",
			},
		},
		{
			ID:       "headscale",
			Name:     "Headscale",
			Category: "vpn",
			Icon:     "🔗",
			ConfigPaths: []string{
				"/etc/headscale",
			},
		},
		{
			ID:       "firewalld",
			Name:     "firewalld",
			Category: "firewall",
			Icon:     "🔥",
			ConfigPaths: []string{
				"/etc/firewalld",
			},
		},
		{
			ID:       "ufw",
			Name:     "UFW",
			Category: "firewall",
			Icon:     "🔥",
			ConfigPaths: []string{
				"/etc/ufw",
			},
		},
		{
			ID:       "nftables",
			Name:     "nftables",
			Category: "firewall",
			Icon:     "🔥",
			ConfigPaths: []string{
				"/etc/nftables.conf",
			},
		},
		{
			ID:       "iptables",
			Name:     "iptables",
			Category: "firewall",
			Icon:     "🔥",
			ConfigPaths: []string{
				"/etc/iptables",
			},
		},
		{
			ID:       "fail2ban",
			Name:     "Fail2ban",
			Category: "security",
			Icon:     "🛡️",
			ConfigPaths: []string{
				"/etc/fail2ban",
			},
		},
		{
			ID:       "crowdsec",
			Name:     "CrowdSec",
			Category: "security",
			Icon:     "🛡️",
			ConfigPaths: []string{
				"/etc/crowdsec",
			},
		},
		{
			ID:       "sshguard",
			Name:     "SSHGuard",
			Category: "security",
			Icon:     "🛡️",
			ConfigPaths: []string{
				"/etc/sshguard",
			},
		},
		{
			ID:       "auditd",
			Name:     "auditd",
			Category: "security",
			Icon:     "📝",
			ConfigPaths: []string{
				"/etc/audit",
			},
		},
		{
			ID:       "apparmor",
			Name:     "AppArmor",
			Category: "security",
			Icon:     "🛡️",
			ConfigPaths: []string{
				"/etc/apparmor.d",
			},
		},
		{
			ID:       "selinux",
			Name:     "SELinux",
			Category: "security",
			Icon:     "🛡️",
			ConfigPaths: []string{
				"/etc/selinux",
			},
		},
		{
			ID:       "firejail",
			Name:     "Firejail",
			Category: "security",
			Icon:     "🔒",
			ConfigPaths: []string{
				"~/.config/firejail",
				"/etc/firejail",
			},
		},
		{
			ID:       "bubblewrap",
			Name:     "Bubblewrap",
			Category: "security",
			Icon:     "🔒",
			ConfigPaths: []string{
				"~/.config/bubblewrap",
			},
		},
		{
			ID:       "flatpak",
			Name:     "Flatpak",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/flatpak",
				"/etc/flatpak",
			},
		},
		{
			ID:       "snap",
			Name:     "Snap",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/snap",
			},
		},
		{
			ID:       "appimage",
			Name:     "AppImage",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/appimagekit",
			},
		},
		{
			ID:       "nala",
			Name:     "Nala",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/nala",
			},
		},
		{
			ID:       "pacman",
			Name:     "Pacman",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/pacman.conf",
				"/etc/pacman.d",
			},
		},
		{
			ID:       "paru",
			Name:     "Paru",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/paru",
			},
		},
		{
			ID:       "yay",
			Name:     "Yay",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/yay",
			},
		},
		{
			ID:       "pikaur",
			Name:     "Pikaur",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/pikaur",
			},
		},
		{
			ID:       "aura",
			Name:     "Aura",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/aura",
			},
		},
		{
			ID:       "pamac",
			Name:     "Pamac",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/pamac.conf",
			},
		},
		{
			ID:       "portage",
			Name:     "Portage",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/portage",
			},
		},
		{
			ID:       "xbps",
			Name:     "XBPS",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/xbps.d",
			},
		},
		{
			ID:       "zypper",
			Name:     "Zypper",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/zypp",
			},
		},
		{
			ID:       "dnf",
			Name:     "DNF",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/dnf",
			},
		},
		{
			ID:       "apt",
			Name:     "APT",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/apt",
			},
		},
		{
			ID:       "apk",
			Name:     "APK",
			Category: "package",
			Icon:     "📦",
			ConfigPaths: []string{
				"/etc/apk",
			},
		},
		{
			ID:       "cron",
			Name:     "Cron",
			Category: "scheduler",
			Icon:     "⏰",
			ConfigPaths: []string{
				"/etc/crontab",
				"/etc/cron.d",
			},
		},
		{
			ID:       "anacron",
			Name:     "Anacron",
			Category: "scheduler",
			Icon:     "⏰",
			ConfigPaths: []string{
				"/etc/anacrontab",
			},
		},
		{
			ID:       "fcron",
			Name:     "Fcron",
			Category: "scheduler",
			Icon:     "⏰",
			ConfigPaths: []string{
				"/etc/fcron.conf",
			},
		},
		{
			ID:       "at",
			Name:     "at",
			Category: "scheduler",
			Icon:     "⏰",
			ConfigPaths: []string{
				"/etc/at.deny",
			},
		},
		{
			ID:       "logrotate",
			Name:     "Logrotate",
			Category: "logging",
			Icon:     "📝",
			ConfigPaths: []string{
				"/etc/logrotate.conf",
				"/etc/logrotate.d",
			},
		},
		{
			ID:       "rsyslog",
			Name:     "rsyslog",
			Category: "logging",
			Icon:     "📝",
			ConfigPaths: []string{
				"/etc/rsyslog.conf",
				"/etc/rsyslog.d",
			},
		},
		{
			ID:       "syslog-ng",
			Name:     "syslog-ng",
			Category: "logging",
			Icon:     "📝",
			ConfigPaths: []string{
				"/etc/syslog-ng",
			},
		},
		{
			ID:       "journald",
			Name:     "journald",
			Category: "logging",
			Icon:     "📝",
			ConfigPaths: []string{
				"/etc/systemd/journald.conf",
			},
		},
		{
			ID:       "loki",
			Name:     "Loki",
			Category: "logging",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/loki",
			},
		},
		{
			ID:       "promtail",
			Name:     "Promtail",
			Category: "logging",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/promtail",
			},
		},
		// More apps (450+)
		{
			ID:       "prometheus",
			Name:     "Prometheus",
			Category: "monitoring",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/prometheus",
			},
		},
		{
			ID:       "grafana",
			Name:     "Grafana",
			Category: "monitoring",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/grafana",
			},
		},
		{
			ID:       "alertmanager",
			Name:     "Alertmanager",
			Category: "monitoring",
			Icon:     "🔔",
			ConfigPaths: []string{
				"/etc/alertmanager",
			},
		},
		{
			ID:       "node_exporter",
			Name:     "Node Exporter",
			Category: "monitoring",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/node_exporter",
			},
		},
		{
			ID:       "cadvisor",
			Name:     "cAdvisor",
			Category: "monitoring",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/cadvisor",
			},
		},
		{
			ID:       "netdata",
			Name:     "Netdata",
			Category: "monitoring",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/netdata",
			},
		},
		{
			ID:       "glances",
			Name:     "Glances",
			Category: "monitoring",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/glances",
			},
		},
		{
			ID:       "vector",
			Name:     "Vector",
			Category: "logging",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/vector",
			},
		},
		{
			ID:       "fluent-bit",
			Name:     "Fluent Bit",
			Category: "logging",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/fluent-bit",
			},
		},
		{
			ID:       "fluentd",
			Name:     "Fluentd",
			Category: "logging",
			Icon:     "📊",
			ConfigPaths: []string{
				"/etc/fluentd",
			},
		},
		{
			ID:       "nginx",
			Name:     "Nginx",
			Category: "webserver",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/nginx",
			},
		},
		{
			ID:       "apache",
			Name:     "Apache",
			Category: "webserver",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/apache2",
				"/etc/httpd",
			},
		},
		{
			ID:       "caddy",
			Name:     "Caddy",
			Category: "webserver",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/caddy",
				"~/.config/caddy",
			},
		},
		{
			ID:       "traefik",
			Name:     "Traefik",
			Category: "webserver",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/traefik",
			},
		},
		{
			ID:       "haproxy",
			Name:     "HAProxy",
			Category: "webserver",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/haproxy",
			},
		},
		{
			ID:       "squid",
			Name:     "Squid",
			Category: "proxy",
			Icon:     "🦑",
			ConfigPaths: []string{
				"/etc/squid",
			},
		},
		{
			ID:       "privoxy",
			Name:     "Privoxy",
			Category: "proxy",
			Icon:     "🔒",
			ConfigPaths: []string{
				"/etc/privoxy",
			},
		},
		{
			ID:       "tor",
			Name:     "Tor",
			Category: "privacy",
			Icon:     "🧅",
			ConfigPaths: []string{
				"/etc/tor",
			},
		},
		{
			ID:       "i2p",
			Name:     "I2P",
			Category: "privacy",
			Icon:     "🔒",
			ConfigPaths: []string{
				"~/.i2p",
			},
		},
		{
			ID:       "postgresql",
			Name:     "PostgreSQL",
			Category: "database",
			Icon:     "🐘",
			ConfigPaths: []string{
				"/etc/postgresql",
			},
		},
		{
			ID:       "mysql",
			Name:     "MySQL",
			Category: "database",
			Icon:     "🐬",
			ConfigPaths: []string{
				"/etc/mysql",
			},
		},
		{
			ID:       "mariadb",
			Name:     "MariaDB",
			Category: "database",
			Icon:     "🐬",
			ConfigPaths: []string{
				"/etc/mysql",
			},
		},
		{
			ID:       "redis",
			Name:     "Redis",
			Category: "database",
			Icon:     "🔴",
			ConfigPaths: []string{
				"/etc/redis",
			},
		},
		{
			ID:       "mongodb",
			Name:     "MongoDB",
			Category: "database",
			Icon:     "🍃",
			ConfigPaths: []string{
				"/etc/mongod.conf",
			},
		},
		{
			ID:       "sqlite",
			Name:     "SQLite",
			Category: "database",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.sqliterc",
			},
		},
		{
			ID:       "cockroachdb",
			Name:     "CockroachDB",
			Category: "database",
			Icon:     "🪳",
			ConfigPaths: []string{
				"/etc/cockroach",
			},
		},
		{
			ID:       "elasticsearch",
			Name:     "Elasticsearch",
			Category: "search",
			Icon:     "🔍",
			ConfigPaths: []string{
				"/etc/elasticsearch",
			},
		},
		{
			ID:       "opensearch",
			Name:     "OpenSearch",
			Category: "search",
			Icon:     "🔍",
			ConfigPaths: []string{
				"/etc/opensearch",
			},
		},
		{
			ID:       "meilisearch",
			Name:     "Meilisearch",
			Category: "search",
			Icon:     "🔍",
			ConfigPaths: []string{
				"/etc/meilisearch",
			},
		},
		{
			ID:       "typesense",
			Name:     "Typesense",
			Category: "search",
			Icon:     "🔍",
			ConfigPaths: []string{
				"/etc/typesense",
			},
		},
		{
			ID:       "rabbitmq",
			Name:     "RabbitMQ",
			Category: "messaging",
			Icon:     "🐰",
			ConfigPaths: []string{
				"/etc/rabbitmq",
			},
		},
		{
			ID:       "kafka",
			Name:     "Kafka",
			Category: "messaging",
			Icon:     "📨",
			ConfigPaths: []string{
				"/etc/kafka",
			},
		},
		{
			ID:       "nats",
			Name:     "NATS",
			Category: "messaging",
			Icon:     "📨",
			ConfigPaths: []string{
				"/etc/nats",
			},
		},
		{
			ID:       "mosquitto",
			Name:     "Mosquitto",
			Category: "messaging",
			Icon:     "🦟",
			ConfigPaths: []string{
				"/etc/mosquitto",
			},
		},
		{
			ID:       "consul",
			Name:     "Consul",
			Category: "service-discovery",
			Icon:     "🔗",
			ConfigPaths: []string{
				"/etc/consul.d",
			},
		},
		{
			ID:       "vault",
			Name:     "Vault",
			Category: "secrets",
			Icon:     "🔐",
			ConfigPaths: []string{
				"/etc/vault.d",
			},
		},
		{
			ID:       "etcd",
			Name:     "etcd",
			Category: "service-discovery",
			Icon:     "🔑",
			ConfigPaths: []string{
				"/etc/etcd",
			},
		},
		{
			ID:       "nomad",
			Name:     "Nomad",
			Category: "orchestration",
			Icon:     "🚀",
			ConfigPaths: []string{
				"/etc/nomad.d",
			},
		},
		{
			ID:       "packer",
			Name:     "Packer",
			Category: "devops",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/packer",
			},
		},
		{
			ID:       "vagrant",
			Name:     "Vagrant",
			Category: "devops",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.vagrant.d",
			},
		},
		{
			ID:       "ansible",
			Name:     "Ansible",
			Category: "devops",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.ansible.cfg",
				"/etc/ansible",
			},
		},
		{
			ID:       "puppet",
			Name:     "Puppet",
			Category: "devops",
			Icon:     "🎭",
			ConfigPaths: []string{
				"/etc/puppetlabs",
			},
		},
		{
			ID:       "chef",
			Name:     "Chef",
			Category: "devops",
			Icon:     "👨‍🍳",
			ConfigPaths: []string{
				"~/.chef",
			},
		},
		{
			ID:       "saltstack",
			Name:     "SaltStack",
			Category: "devops",
			Icon:     "🧂",
			ConfigPaths: []string{
				"/etc/salt",
			},
		},
		{
			ID:       "pulumi",
			Name:     "Pulumi",
			Category: "devops",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.pulumi",
			},
		},
		{
			ID:       "cdktf",
			Name:     "CDKTF",
			Category: "devops",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.cdktf",
			},
		},
		// More apps (500+)
		{
			ID:       "opentofu",
			Name:     "OpenTofu",
			Category: "devops",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.config/opentofu",
			},
		},
		{
			ID:       "terragrunt",
			Name:     "Terragrunt",
			Category: "devops",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.terragrunt",
			},
		},
		{
			ID:       "atlantis",
			Name:     "Atlantis",
			Category: "devops",
			Icon:     "🌊",
			ConfigPaths: []string{
				"/etc/atlantis",
			},
		},
		{
			ID:       "argocd",
			Name:     "ArgoCD",
			Category: "gitops",
			Icon:     "🐙",
			ConfigPaths: []string{
				"~/.config/argocd",
			},
		},
		{
			ID:       "flux",
			Name:     "Flux",
			Category: "gitops",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/flux",
			},
		},
		{
			ID:       "jenkins",
			Name:     "Jenkins",
			Category: "ci",
			Icon:     "🔧",
			ConfigPaths: []string{
				"/var/lib/jenkins",
			},
		},
		{
			ID:       "drone",
			Name:     "Drone",
			Category: "ci",
			Icon:     "🚁",
			ConfigPaths: []string{
				"/etc/drone",
			},
		},
		{
			ID:       "woodpecker",
			Name:     "Woodpecker",
			Category: "ci",
			Icon:     "🪶",
			ConfigPaths: []string{
				"/etc/woodpecker",
			},
		},
		{
			ID:       "concourse",
			Name:     "Concourse",
			Category: "ci",
			Icon:     "✈️",
			ConfigPaths: []string{
				"/etc/concourse",
			},
		},
		{
			ID:       "buildkite",
			Name:     "Buildkite",
			Category: "ci",
			Icon:     "🏗️",
			ConfigPaths: []string{
				"~/.buildkite",
			},
		},
		{
			ID:       "circleci",
			Name:     "CircleCI",
			Category: "ci",
			Icon:     "⭕",
			ConfigPaths: []string{
				"~/.circleci",
			},
		},
		{
			ID:       "dagger",
			Name:     "Dagger",
			Category: "ci",
			Icon:     "🗡️",
			ConfigPaths: []string{
				"~/.config/dagger",
			},
		},
		{
			ID:       "earthly",
			Name:     "Earthly",
			Category: "ci",
			Icon:     "🌍",
			ConfigPaths: []string{
				"~/.earthly",
			},
		},
		{
			ID:       "bazel",
			Name:     "Bazel",
			Category: "build",
			Icon:     "🏗️",
			ConfigPaths: []string{
				"~/.bazelrc",
			},
		},
		{
			ID:       "buck2",
			Name:     "Buck2",
			Category: "build",
			Icon:     "🏗️",
			ConfigPaths: []string{
				"~/.buckconfig",
			},
		},
		{
			ID:       "pants",
			Name:     "Pants",
			Category: "build",
			Icon:     "👖",
			ConfigPaths: []string{
				"pants.toml",
			},
		},
		{
			ID:       "please",
			Name:     "Please",
			Category: "build",
			Icon:     "🏗️",
			ConfigPaths: []string{
				".plzconfig",
			},
		},
		{
			ID:       "gradle-enterprise",
			Name:     "Gradle Enterprise",
			Category: "build",
			Icon:     "🐘",
			ConfigPaths: []string{
				"~/.gradle",
			},
		},
		{
			ID:       "sbt",
			Name:     "sbt",
			Category: "build",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.sbt",
			},
		},
		{
			ID:       "mill",
			Name:     "Mill",
			Category: "build",
			Icon:     "🏭",
			ConfigPaths: []string{
				"~/.mill",
			},
		},
		{
			ID:       "leiningen",
			Name:     "Leiningen",
			Category: "build",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.lein",
			},
		},
		{
			ID:       "boot",
			Name:     "Boot",
			Category: "build",
			Icon:     "👢",
			ConfigPaths: []string{
				"~/.boot",
			},
		},
		{
			ID:       "mix",
			Name:     "Mix",
			Category: "build",
			Icon:     "🧪",
			ConfigPaths: []string{
				"~/.mix",
			},
		},
		{
			ID:       "rebar3",
			Name:     "Rebar3",
			Category: "build",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.config/rebar3",
			},
		},
		{
			ID:       "cabal",
			Name:     "Cabal",
			Category: "build",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.cabal",
			},
		},
		{
			ID:       "stack",
			Name:     "Stack",
			Category: "build",
			Icon:     "📚",
			ConfigPaths: []string{
				"~/.stack",
			},
		},
		{
			ID:       "opam",
			Name:     "opam",
			Category: "build",
			Icon:     "🐫",
			ConfigPaths: []string{
				"~/.opam",
			},
		},
		{
			ID:       "dune",
			Name:     "Dune",
			Category: "build",
			Icon:     "🏜️",
			ConfigPaths: []string{
				"~/.config/dune",
			},
		},
		{
			ID:       "nimble",
			Name:     "Nimble",
			Category: "build",
			Icon:     "👑",
			ConfigPaths: []string{
				"~/.config/nimble",
			},
		},
		{
			ID:       "zig",
			Name:     "Zig",
			Category: "build",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/zig",
			},
		},
		{
			ID:       "odin",
			Name:     "Odin",
			Category: "build",
			Icon:     "🔷",
			ConfigPaths: []string{
				"~/.config/odin",
			},
		},
		{
			ID:       "v",
			Name:     "V",
			Category: "build",
			Icon:     "✅",
			ConfigPaths: []string{
				"~/.vlang",
			},
		},
		{
			ID:       "crystal",
			Name:     "Crystal",
			Category: "build",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.crystal",
			},
		},
		{
			ID:       "julia",
			Name:     "Julia",
			Category: "build",
			Icon:     "🔮",
			ConfigPaths: []string{
				"~/.julia",
			},
		},
		{
			ID:       "r",
			Name:     "R",
			Category: "build",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.Rprofile",
				"~/.Renviron",
			},
		},
		{
			ID:       "octave",
			Name:     "GNU Octave",
			Category: "build",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.octaverc",
			},
		},
		{
			ID:       "maxima",
			Name:     "Maxima",
			Category: "build",
			Icon:     "🧮",
			ConfigPaths: []string{
				"~/.maxima",
			},
		},
		{
			ID:       "sage",
			Name:     "SageMath",
			Category: "build",
			Icon:     "🧮",
			ConfigPaths: []string{
				"~/.sage",
			},
		},
		{
			ID:       "gap",
			Name:     "GAP",
			Category: "build",
			Icon:     "🧮",
			ConfigPaths: []string{
				"~/.gap",
			},
		},
		{
			ID:       "coq",
			Name:     "Coq",
			Category: "build",
			Icon:     "🐓",
			ConfigPaths: []string{
				"~/.coqrc",
			},
		},
		{
			ID:       "lean",
			Name:     "Lean",
			Category: "build",
			Icon:     "📐",
			ConfigPaths: []string{
				"~/.elan",
			},
		},
		{
			ID:       "agda",
			Name:     "Agda",
			Category: "build",
			Icon:     "📐",
			ConfigPaths: []string{
				"~/.agda",
			},
		},
		{
			ID:       "idris",
			Name:     "Idris",
			Category: "build",
			Icon:     "🐉",
			ConfigPaths: []string{
				"~/.idris2",
			},
		},
		{
			ID:       "racket",
			Name:     "Racket",
			Category: "build",
			Icon:     "🎾",
			ConfigPaths: []string{
				"~/.racket",
			},
		},
		{
			ID:       "guile",
			Name:     "Guile",
			Category: "build",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.guile",
			},
		},
		{
			ID:       "chicken",
			Name:     "Chicken Scheme",
			Category: "build",
			Icon:     "🐔",
			ConfigPaths: []string{
				"~/.chicken",
			},
		},
		{
			ID:       "sbcl",
			Name:     "SBCL",
			Category: "build",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.sbclrc",
			},
		},
		{
			ID:       "clisp",
			Name:     "CLISP",
			Category: "build",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.clisprc",
			},
		},
		{
			ID:       "ecl",
			Name:     "ECL",
			Category: "build",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.eclrc",
			},
		},
		{
			ID:       "abcl",
			Name:     "ABCL",
			Category: "build",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.abclrc",
			},
		},
		{
			ID:       "ccl",
			Name:     "CCL",
			Category: "build",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.ccl-init.lisp",
			},
		},
		// More apps (550+)
		{
			ID:       "allegro",
			Name:     "Allegro CL",
			Category: "lisp",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.clinit.cl",
			},
		},
		{
			ID:       "lispworks",
			Name:     "LispWorks",
			Category: "lisp",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.lispworks",
			},
		},
		{
			ID:       "clojure",
			Name:     "Clojure",
			Category: "lisp",
			Icon:     "🟢",
			ConfigPaths: []string{
				"~/.clojure",
			},
		},
		{
			ID:       "babashka",
			Name:     "Babashka",
			Category: "lisp",
			Icon:     "🟢",
			ConfigPaths: []string{
				"~/.config/babashka",
			},
		},
		{
			ID:       "hy",
			Name:     "Hy",
			Category: "lisp",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/hy",
			},
		},
		{
			ID:       "fennel",
			Name:     "Fennel",
			Category: "lisp",
			Icon:     "🌿",
			ConfigPaths: []string{
				"~/.fennelrc",
			},
		},
		{
			ID:       "janet",
			Name:     "Janet",
			Category: "lisp",
			Icon:     "🔮",
			ConfigPaths: []string{
				"~/.janet",
			},
		},
		{
			ID:       "picolisp",
			Name:     "PicoLisp",
			Category: "lisp",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.pil",
			},
		},
		{
			ID:       "newlisp",
			Name:     "newLISP",
			Category: "lisp",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.init.lsp",
			},
		},
		{
			ID:       "forth",
			Name:     "Forth",
			Category: "language",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.forth",
			},
		},
		{
			ID:       "gforth",
			Name:     "Gforth",
			Category: "language",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.gforth",
			},
		},
		{
			ID:       "factor",
			Name:     "Factor",
			Category: "language",
			Icon:     "🔢",
			ConfigPaths: []string{
				"~/.factor",
			},
		},
		{
			ID:       "red",
			Name:     "Red",
			Category: "language",
			Icon:     "🔴",
			ConfigPaths: []string{
				"~/.red",
			},
		},
		{
			ID:       "rebol",
			Name:     "REBOL",
			Category: "language",
			Icon:     "🔴",
			ConfigPaths: []string{
				"~/.rebol",
			},
		},
		{
			ID:       "io",
			Name:     "Io",
			Category: "language",
			Icon:     "💫",
			ConfigPaths: []string{
				"~/.io",
			},
		},
		{
			ID:       "wren",
			Name:     "Wren",
			Category: "language",
			Icon:     "🐦",
			ConfigPaths: []string{
				"~/.wren",
			},
		},
		{
			ID:       "gravity",
			Name:     "Gravity",
			Category: "language",
			Icon:     "🍎",
			ConfigPaths: []string{
				"~/.gravity",
			},
		},
		{
			ID:       "squirrel",
			Name:     "Squirrel",
			Category: "language",
			Icon:     "🐿️",
			ConfigPaths: []string{
				"~/.squirrel",
			},
		},
		{
			ID:       "angelscript",
			Name:     "AngelScript",
			Category: "language",
			Icon:     "😇",
			ConfigPaths: []string{
				"~/.angelscript",
			},
		},
		{
			ID:       "chaiscript",
			Name:     "ChaiScript",
			Category: "language",
			Icon:     "☕",
			ConfigPaths: []string{
				"~/.chaiscript",
			},
		},
		{
			ID:       "mruby",
			Name:     "mruby",
			Category: "ruby",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.mrb",
			},
		},
		{
			ID:       "jruby",
			Name:     "JRuby",
			Category: "ruby",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.jruby",
			},
		},
		{
			ID:       "truffleruby",
			Name:     "TruffleRuby",
			Category: "ruby",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.truffleruby",
			},
		},
		{
			ID:       "rbenv",
			Name:     "rbenv",
			Category: "ruby",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.rbenv",
			},
		},
		{
			ID:       "rvm",
			Name:     "RVM",
			Category: "ruby",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.rvm",
			},
		},
		{
			ID:       "chruby",
			Name:     "chruby",
			Category: "ruby",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.ruby-version",
			},
		},
		{
			ID:       "pyenv",
			Name:     "pyenv",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.pyenv",
			},
		},
		{
			ID:       "pyright",
			Name:     "Pyright",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/pyright",
			},
		},
		{
			ID:       "mypy",
			Name:     "mypy",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/mypy",
			},
		},
		{
			ID:       "black",
			Name:     "Black",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/black",
			},
		},
		{
			ID:       "isort",
			Name:     "isort",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.isort.cfg",
			},
		},
		{
			ID:       "flake8",
			Name:     "Flake8",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/flake8",
			},
		},
		{
			ID:       "bandit",
			Name:     "Bandit",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/bandit",
			},
		},
		{
			ID:       "pytype",
			Name:     "Pytype",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/pytype",
			},
		},
		{
			ID:       "pipx",
			Name:     "pipx",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.local/pipx",
			},
		},
		{
			ID:       "pdm",
			Name:     "PDM",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/pdm",
			},
		},
		{
			ID:       "hatch",
			Name:     "Hatch",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/hatch",
			},
		},
		{
			ID:       "nvm",
			Name:     "NVM",
			Category: "node",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.nvm",
			},
		},
		{
			ID:       "fnm",
			Name:     "fnm",
			Category: "node",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/fnm",
			},
		},
		{
			ID:       "volta",
			Name:     "Volta",
			Category: "node",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.volta",
			},
		},
		{
			ID:       "n",
			Name:     "n",
			Category: "node",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.n",
			},
		},
		{
			ID:       "biome",
			Name:     "Biome",
			Category: "node",
			Icon:     "🌿",
			ConfigPaths: []string{
				"~/.config/biome",
			},
		},
		{
			ID:       "oxlint",
			Name:     "oxlint",
			Category: "node",
			Icon:     "🐂",
			ConfigPaths: []string{
				"~/.config/oxlint",
			},
		},
		{
			ID:       "rome",
			Name:     "Rome",
			Category: "node",
			Icon:     "🏛️",
			ConfigPaths: []string{
				"~/.config/rome",
			},
		},
		{
			ID:       "swc",
			Name:     "SWC",
			Category: "node",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.swcrc",
			},
		},
		{
			ID:       "esbuild",
			Name:     "esbuild",
			Category: "node",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/esbuild",
			},
		},
		{
			ID:       "turbo",
			Name:     "Turborepo",
			Category: "node",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.turbo",
			},
		},
		{
			ID:       "nx",
			Name:     "Nx",
			Category: "node",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.nx",
			},
		},
		{
			ID:       "lerna",
			Name:     "Lerna",
			Category: "node",
			Icon:     "🐉",
			ConfigPaths: []string{
				"~/.lerna",
			},
		},
		{
			ID:       "rush",
			Name:     "Rush",
			Category: "node",
			Icon:     "🏃",
			ConfigPaths: []string{
				"~/.rush",
			},
		},

		// Changesets (monorepo versioning)
		{
			ID:       "changesets",
			Name:     "Changesets",
			Category: "node",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.changeset",
			},
		},

		// Verdaccio (private npm registry)
		{
			ID:       "verdaccio",
			Name:     "Verdaccio",
			Category: "node",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/verdaccio",
			},
		},

		// ========== AI/ML Tools ==========

		// Ollama (local LLM)
		{
			ID:       "ollama",
			Name:     "Ollama",
			Category: "ai",
			Icon:     "🦙",
			ConfigPaths: []string{
				"~/.ollama",
			},
		},

		// LM Studio
		{
			ID:       "lmstudio",
			Name:     "LM Studio",
			Category: "ai",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.cache/lm-studio",
			},
		},

		// Aider (AI pair programming)
		{
			ID:       "aider",
			Name:     "Aider",
			Category: "ai",
			Icon:     "🤝",
			ConfigPaths: []string{
				"~/.aider.conf.yml",
				"~/.aider",
			},
		},

		// Continue.dev (AI code assistant)
		{
			ID:       "continue",
			Name:     "Continue",
			Category: "ai",
			Icon:     "➡️",
			ConfigPaths: []string{
				"~/.continue",
			},
		},

		// OpenAI CLI
		{
			ID:       "openai",
			Name:     "OpenAI CLI",
			Category: "ai",
			Icon:     "🧠",
			ConfigPaths: []string{
				"~/.config/openai",
			},
		},

		// ========== Container & Kubernetes ==========

		// Podman Desktop
		{
			ID:       "podman-desktop",
			Name:     "Podman Desktop",
			Category: "container",
			Icon:     "🦭",
			ConfigPaths: []string{
				"~/.config/podman-desktop",
			},
		},

		// Rancher Desktop
		{
			ID:       "rancher-desktop",
			Name:     "Rancher Desktop",
			Category: "container",
			Icon:     "🐄",
			ConfigPaths: []string{
				"~/.config/rancher-desktop",
			},
		},

		// Lima (Linux VMs on macOS)
		{
			ID:       "lima",
			Name:     "Lima",
			Category: "container",
			Icon:     "🐧",
			ConfigPaths: []string{
				"~/.lima",
			},
		},

		// Finch (container development)
		{
			ID:       "finch",
			Name:     "Finch",
			Category: "container",
			Icon:     "🐦",
			ConfigPaths: []string{
				"~/.finch",
			},
		},

		// Skaffold (Kubernetes workflow)
		{
			ID:       "skaffold",
			Name:     "Skaffold",
			Category: "kubernetes",
			Icon:     "⛵",
			ConfigPaths: []string{
				"~/.skaffold",
			},
		},

		// Kustomize
		{
			ID:       "kustomize",
			Name:     "Kustomize",
			Category: "kubernetes",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/kustomize",
			},
		},

		// Stern (multi-pod log tailing)
		{
			ID:       "stern",
			Name:     "Stern",
			Category: "kubernetes",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/stern",
			},
		},

		// Kubectx/Kubens
		{
			ID:       "kubectx",
			Name:     "Kubectx",
			Category: "kubernetes",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/kubectx",
			},
		},

		// ========== Modern CLI Tools ==========

		// Carapace (shell completions)
		{
			ID:       "carapace",
			Name:     "Carapace",
			Category: "shell",
			Icon:     "🐢",
			ConfigPaths: []string{
				"~/.config/carapace",
			},
		},

		// Sheldon (shell plugin manager)
		{
			ID:       "sheldon",
			Name:     "Sheldon",
			Category: "shell",
			Icon:     "🔌",
			ConfigPaths: []string{
				"~/.config/sheldon",
			},
		},

		// ========== Git Tools ==========

		// Git-cliff (changelog generator)
		{
			ID:       "git-cliff",
			Name:     "Git Cliff",
			Category: "git",
			Icon:     "🏔️",
			ConfigPaths: []string{
				"~/.config/git-cliff",
			},
		},

		// ========== Documentation Tools ==========

		// mdBook
		{
			ID:       "mdbook",
			Name:     "mdBook",
			Category: "docs",
			Icon:     "📚",
			ConfigPaths: []string{
				"~/.config/mdbook",
			},
		},

		// Docusaurus
		{
			ID:       "docusaurus",
			Name:     "Docusaurus",
			Category: "docs",
			Icon:     "🦖",
			ConfigPaths: []string{
				"~/.docusaurus",
			},
		},

		// ========== API & Testing Tools ==========

		// Bruno (API client)
		{
			ID:       "bruno",
			Name:     "Bruno",
			Category: "api",
			Icon:     "🐶",
			ConfigPaths: []string{
				"~/.config/bruno",
			},
		},

		// Hurl (HTTP testing)
		{
			ID:       "hurl",
			Name:     "Hurl",
			Category: "api",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/hurl",
			},
		},

		// k6 (load testing)
		{
			ID:       "k6",
			Name:     "k6",
			Category: "testing",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/k6",
			},
		},

		// ========== Database Tools ==========

		// DBeaver
		{
			ID:       "dbeaver",
			Name:     "DBeaver",
			Category: "database",
			Icon:     "🦫",
			ConfigPaths: []string{
				"~/.dbeaver",
				"~/.config/dbeaver",
			},
		},

		// usql (universal SQL client)
		{
			ID:       "usql",
			Name:     "usql",
			Category: "database",
			Icon:     "🔌",
			ConfigPaths: []string{
				"~/.config/usql",
			},
		},

		// ========== Terminal Multiplexers ==========

		// Byobu
		{
			ID:       "byobu",
			Name:     "Byobu",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.byobu",
			},
		},

		// ========== Note Taking ==========

		// Joplin
		{
			ID:       "joplin",
			Name:     "Joplin",
			Category: "notes",
			Icon:     "📒",
			ConfigPaths: []string{
				"~/.config/joplin",
				"~/.config/joplin-desktop",
			},
		},

		// ========== Music & Media ==========

		// Spotify TUI
		{
			ID:       "spotify-tui",
			Name:     "Spotify TUI",
			Category: "media",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.config/spotify-tui",
			},
		},

		// ========== Fonts & Theming ==========

		// Fontconfig
		{
			ID:       "fontconfig",
			Name:     "Fontconfig",
			Category: "system",
			Icon:     "🔤",
			ConfigPaths: []string{
				"~/.config/fontconfig",
			},
		},

		// GTK
		{
			ID:       "gtk",
			Name:     "GTK",
			Category: "system",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/gtk-3.0",
				"~/.config/gtk-4.0",
				"~/.gtkrc-2.0",
			},
		},

		// Qt
		{
			ID:       "qt",
			Name:     "Qt",
			Category: "system",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/qt5ct",
				"~/.config/qt6ct",
			},
		},

		// ========== System Utilities ==========

		// Timeshift
		{
			ID:       "timeshift",
			Name:     "Timeshift",
			Category: "backup",
			Icon:     "⏰",
			ConfigPaths: []string{
				"~/.config/timeshift",
			},
		},

		// Vorta (BorgBackup frontend)
		{
			ID:       "vorta",
			Name:     "Vorta",
			Category: "backup",
			Icon:     "💾",
			ConfigPaths: []string{
				"~/.config/vorta",
			},
		},

		// ========== Network Tools ==========

		// Charles Proxy
		{
			ID:       "charles",
			Name:     "Charles Proxy",
			Category: "network",
			Icon:     "🕵️",
			ConfigPaths: []string{
				"~/.charles",
			},
		},

		// Proxyman
		{
			ID:       "proxyman",
			Name:     "Proxyman",
			Category: "network",
			Icon:     "🔌",
			ConfigPaths: []string{
				"~/.config/proxyman",
			},
		},

		// ========== macOS Productivity ==========

		// Alfred
		{
			ID:       "alfred",
			Name:     "Alfred",
			Category: "productivity",
			Icon:     "🎩",
			ConfigPaths: []string{
				"~/.alfred",
			},
		},

		// BetterTouchTool
		{
			ID:       "bettertouchtool",
			Name:     "BetterTouchTool",
			Category: "productivity",
			Icon:     "👆",
			ConfigPaths: []string{
				"~/.config/bettertouchtool",
			},
		},

		// Keyboard Maestro
		{
			ID:       "keyboard-maestro",
			Name:     "Keyboard Maestro",
			Category: "productivity",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.keyboard-maestro",
			},
		},

		// ========== Code Quality ==========

		// SonarLint
		{
			ID:       "sonarlint",
			Name:     "SonarLint",
			Category: "linter",
			Icon:     "🔊",
			ConfigPaths: []string{
				"~/.sonarlint",
			},
		},

		// Semgrep
		{
			ID:       "semgrep",
			Name:     "Semgrep",
			Category: "security",
			Icon:     "🔎",
			ConfigPaths: []string{
				"~/.semgrep",
			},
		},

		// Trivy
		{
			ID:       "trivy",
			Name:     "Trivy",
			Category: "security",
			Icon:     "🛡️",
			ConfigPaths: []string{
				"~/.trivy",
				"~/.config/trivy",
			},
		},

		// ========== Rust Ecosystem ==========

		// Rustup
		{
			ID:       "rustup",
			Name:     "Rustup",
			Category: "rust",
			Icon:     "🦀",
			ConfigPaths: []string{
				"~/.rustup",
			},
		},

		// rust-analyzer
		{
			ID:       "rust-analyzer",
			Name:     "rust-analyzer",
			Category: "rust",
			Icon:     "🔬",
			ConfigPaths: []string{
				"~/.config/rust-analyzer",
			},
		},

		// ========== Java Ecosystem ==========

		// SDKMAN
		{
			ID:       "sdkman",
			Name:     "SDKMAN",
			Category: "java",
			Icon:     "☕",
			ConfigPaths: []string{
				"~/.sdkman",
			},
		},

		// jEnv
		{
			ID:       "jenv",
			Name:     "jEnv",
			Category: "java",
			Icon:     "☕",
			ConfigPaths: []string{
				"~/.jenv",
			},
		},

		// ========== Go Ecosystem ==========

		// gopls
		{
			ID:       "gopls",
			Name:     "gopls",
			Category: "go",
			Icon:     "🐹",
			ConfigPaths: []string{
				"~/.config/gopls",
			},
		},

		// golangci-lint
		{
			ID:       "golangci-lint",
			Name:     "golangci-lint",
			Category: "go",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.golangci.yml",
				"~/.config/golangci-lint",
			},
		},

		// ========== Mobile Development ==========

		// Flutter
		{
			ID:       "flutter",
			Name:     "Flutter",
			Category: "mobile",
			Icon:     "🦋",
			ConfigPaths: []string{
				"~/.flutter",
				"~/.config/flutter",
			},
		},

		// Android Studio
		{
			ID:       "android-studio",
			Name:     "Android Studio",
			Category: "mobile",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.android",
			},
		},

		// Xcode
		{
			ID:       "xcode",
			Name:     "Xcode",
			Category: "mobile",
			Icon:     "🔨",
			ConfigPaths: []string{
				"~/.xcode",
			},
		},

		// CocoaPods
		{
			ID:       "cocoapods",
			Name:     "CocoaPods",
			Category: "mobile",
			Icon:     "🍫",
			ConfigPaths: []string{
				"~/.cocoapods",
			},
		},

		// ========== Data Science ==========

		// Jupyter
		{
			ID:       "jupyter",
			Name:     "Jupyter",
			Category: "data",
			Icon:     "📓",
			ConfigPaths: []string{
				"~/.jupyter",
			},
		},

		// Conda
		{
			ID:       "conda",
			Name:     "Conda",
			Category: "data",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.condarc",
				"~/.conda",
			},
		},

		// Mamba
		{
			ID:       "mamba",
			Name:     "Mamba",
			Category: "data",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.mambarc",
			},
		},

		// ========== Game Development ==========

		// Godot
		{
			ID:       "godot",
			Name:     "Godot",
			Category: "gamedev",
			Icon:     "🎮",
			ConfigPaths: []string{
				"~/.config/godot",
			},
		},

		// Unity
		{
			ID:       "unity",
			Name:     "Unity",
			Category: "gamedev",
			Icon:     "🎮",
			ConfigPaths: []string{
				"~/.config/unity3d",
			},
		},

		// ========== Design Tools ==========

		// Figma
		{
			ID:       "figma",
			Name:     "Figma",
			Category: "design",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/figma",
			},
		},

		// GIMP
		{
			ID:       "gimp",
			Name:     "GIMP",
			Category: "design",
			Icon:     "🖌️",
			ConfigPaths: []string{
				"~/.config/GIMP",
				"~/.gimp",
			},
		},

		// Inkscape
		{
			ID:       "inkscape",
			Name:     "Inkscape",
			Category: "design",
			Icon:     "✏️",
			ConfigPaths: []string{
				"~/.config/inkscape",
			},
		},

		// ImageMagick
		{
			ID:       "imagemagick",
			Name:     "ImageMagick",
			Category: "design",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/ImageMagick",
			},
		},

		// ========== Video & Audio ==========

		// FFmpeg
		{
			ID:       "ffmpeg",
			Name:     "FFmpeg",
			Category: "media",
			Icon:     "🎬",
			ConfigPaths: []string{
				"~/.ffmpeg",
			},
		},

		// OBS Studio
		{
			ID:       "obs",
			Name:     "OBS Studio",
			Category: "media",
			Icon:     "📹",
			ConfigPaths: []string{
				"~/.config/obs-studio",
			},
		},

		// Audacity
		{
			ID:       "audacity",
			Name:     "Audacity",
			Category: "media",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.audacity-data",
				"~/.config/audacity",
			},
		},

		// VLC
		{
			ID:       "vlc",
			Name:     "VLC",
			Category: "media",
			Icon:     "🎬",
			ConfigPaths: []string{
				"~/.config/vlc",
			},
		},

		// ========== Collaboration ==========

		// Slack
		{
			ID:       "slack",
			Name:     "Slack",
			Category: "communication",
			Icon:     "💬",
			ConfigPaths: []string{
				"~/.config/Slack",
			},
		},

		// Discord
		{
			ID:       "discord",
			Name:     "Discord",
			Category: "communication",
			Icon:     "🎮",
			ConfigPaths: []string{
				"~/.config/discord",
			},
		},

		// Zoom
		{
			ID:       "zoom",
			Name:     "Zoom",
			Category: "communication",
			Icon:     "📹",
			ConfigPaths: []string{
				"~/.zoom",
				"~/.config/zoom",
			},
		},

		// ========== Browser Extensions ==========

		// Vimium
		{
			ID:       "vimium",
			Name:     "Vimium",
			Category: "browser",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.vimium",
			},
		},

		// Surfingkeys
		{
			ID:       "surfingkeys",
			Name:     "Surfingkeys",
			Category: "browser",
			Icon:     "🏄",
			ConfigPaths: []string{
				"~/.surfingkeys",
			},
		},

		// ========== Infrastructure as Code ==========

		// CDK (AWS)
		{
			ID:       "aws-cdk",
			Name:     "AWS CDK",
			Category: "iac",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.cdk",
			},
		},

		// ========== Package Managers ==========

		// Homebrew
		{
			ID:       "homebrew-config",
			Name:     "Homebrew Config",
			Category: "package",
			Icon:     "🍺",
			ConfigPaths: []string{
				"~/.config/homebrew",
			},
		},

		// Nix
		{
			ID:       "nix-config",
			Name:     "Nix Config",
			Category: "package",
			Icon:     "❄️",
			ConfigPaths: []string{
				"~/.config/nix",
				"~/.nixpkgs",
			},
		},

		// ========== Testing Frameworks ==========

		// Jest
		{
			ID:       "jest",
			Name:     "Jest",
			Category: "testing",
			Icon:     "🃏",
			ConfigPaths: []string{
				"~/.jestrc",
			},
		},

		// Pytest
		{
			ID:       "pytest",
			Name:     "Pytest",
			Category: "testing",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.pytest.ini",
				"~/.config/pytest",
			},
		},

		// ========== Code Formatters ==========

		// Prettier
		{
			ID:       "prettier-config",
			Name:     "Prettier",
			Category: "formatter",
			Icon:     "💅",
			ConfigPaths: []string{
				"~/.prettierrc",
				"~/.prettierrc.json",
			},
		},

		// ========== Logging & Tracing ==========

		// Jaeger
		{
			ID:       "jaeger",
			Name:     "Jaeger",
			Category: "observability",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/jaeger",
			},
		},

		// OpenTelemetry
		{
			ID:       "opentelemetry",
			Name:     "OpenTelemetry",
			Category: "observability",
			Icon:     "📡",
			ConfigPaths: []string{
				"~/.config/otel",
			},
		},

		// ========== Secret Management ==========

		// Vault CLI
		{
			ID:       "vault-cli",
			Name:     "Vault CLI",
			Category: "secrets",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.vault-token",
				"~/.config/vault",
			},
		},

		// 1Password CLI
		{
			ID:       "op-cli",
			Name:     "1Password CLI",
			Category: "secrets",
			Icon:     "🔑",
			ConfigPaths: []string{
				"~/.config/op",
			},
		},

		// ========== Task Runners ==========

		// Make
		{
			ID:       "make",
			Name:     "Make",
			Category: "build",
			Icon:     "🔨",
			ConfigPaths: []string{
				"~/.makerc",
			},
		},

		// Taskfile
		{
			ID:       "taskfile",
			Name:     "Taskfile",
			Category: "build",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/task",
			},
		},

		// ========== SSH & Remote ==========

		// Mosh
		{
			ID:       "mosh",
			Name:     "Mosh",
			Category: "remote",
			Icon:     "📡",
			ConfigPaths: []string{
				"~/.mosh",
			},
		},

		// Eternal Terminal
		{
			ID:       "et",
			Name:     "Eternal Terminal",
			Category: "remote",
			Icon:     "🔌",
			ConfigPaths: []string{
				"~/.et",
			},
		},

		// ========== File Sync ==========

		// Syncthing
		{
			ID:       "syncthing-config",
			Name:     "Syncthing",
			Category: "sync",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/syncthing",
			},
		},

		// Unison
		{
			ID:       "unison",
			Name:     "Unison",
			Category: "sync",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.unison",
			},
		},

		// ========== Diagram Tools ==========

		// PlantUML
		{
			ID:       "plantuml",
			Name:     "PlantUML",
			Category: "diagram",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.plantuml",
			},
		},

		// Mermaid
		{
			ID:       "mermaid",
			Name:     "Mermaid",
			Category: "diagram",
			Icon:     "🧜",
			ConfigPaths: []string{
				"~/.mermaid",
			},
		},

		// D2
		{
			ID:       "d2",
			Name:     "D2",
			Category: "diagram",
			Icon:     "📐",
			ConfigPaths: []string{
				"~/.config/d2",
			},
		},

		// ========== Benchmarking ==========

		// Hyperfine
		{
			ID:       "hyperfine-config",
			Name:     "Hyperfine",
			Category: "benchmark",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/hyperfine",
			},
		},

		// wrk
		{
			ID:       "wrk",
			Name:     "wrk",
			Category: "benchmark",
			Icon:     "📈",
			ConfigPaths: []string{
				"~/.wrk",
			},
		},

		// ========== Log Viewers ==========

		// lnav
		{
			ID:       "lnav",
			Name:     "lnav",
			Category: "logs",
			Icon:     "📜",
			ConfigPaths: []string{
				"~/.lnav",
				"~/.config/lnav",
			},
		},

		// multitail
		{
			ID:       "multitail",
			Name:     "multitail",
			Category: "logs",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.multitailrc",
			},
		},

		// ========== Email Clients ==========

		// mbsync (isstringsync)
		{
			ID:       "mbsync",
			Name:     "mbsync",
			Category: "email",
			Icon:     "📧",
			ConfigPaths: []string{
				"~/.mbsyncrc",
			},
		},

		// msmtp
		{
			ID:       "msmtp",
			Name:     "msmtp",
			Category: "email",
			Icon:     "📤",
			ConfigPaths: []string{
				"~/.msmtprc",
				"~/.config/msmtp",
			},
		},

		// notmuch
		{
			ID:       "notmuch",
			Name:     "notmuch",
			Category: "email",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.notmuch-config",
			},
		},

		// ========== Window Management (Linux) ==========

		// Picom
		{
			ID:       "picom-config",
			Name:     "Picom",
			Category: "compositor",
			Icon:     "✨",
			ConfigPaths: []string{
				"~/.config/picom",
			},
		},

		// Dunst
		{
			ID:       "dunst-config",
			Name:     "Dunst",
			Category: "notification",
			Icon:     "🔔",
			ConfigPaths: []string{
				"~/.config/dunst",
			},
		},

		// ========== Finance ==========

		// Ledger
		{
			ID:       "ledger",
			Name:     "Ledger",
			Category: "finance",
			Icon:     "💰",
			ConfigPaths: []string{
				"~/.ledgerrc",
			},
		},

		// hledger
		{
			ID:       "hledger",
			Name:     "hledger",
			Category: "finance",
			Icon:     "💵",
			ConfigPaths: []string{
				"~/.hledger.journal",
			},
		},

		// beancount
		{
			ID:       "beancount",
			Name:     "Beancount",
			Category: "finance",
			Icon:     "🫘",
			ConfigPaths: []string{
				"~/.beancount",
			},
		},

		// ========== Web Frameworks ==========

		// Next.js
		{
			ID:       "nextjs",
			Name:     "Next.js",
			Category: "web",
			Icon:     "▲",
			ConfigPaths: []string{
				"~/.next",
			},
		},

		// Nuxt
		{
			ID:       "nuxt",
			Name:     "Nuxt",
			Category: "web",
			Icon:     "💚",
			ConfigPaths: []string{
				"~/.nuxt",
			},
		},

		// Vite
		{
			ID:       "vite",
			Name:     "Vite",
			Category: "web",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.vite",
			},
		},

		// ========== Static Site Generators ==========

		// Hugo
		{
			ID:       "hugo",
			Name:     "Hugo",
			Category: "ssg",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/hugo",
			},
		},

		// Jekyll
		{
			ID:       "jekyll",
			Name:     "Jekyll",
			Category: "ssg",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.jekyll",
			},
		},

		// Astro
		{
			ID:       "astro",
			Name:     "Astro",
			Category: "ssg",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.astro",
			},
		},

		// ========== Cloud CLIs ==========

		// DigitalOcean
		{
			ID:       "doctl",
			Name:     "DigitalOcean CLI",
			Category: "cloud",
			Icon:     "🌊",
			ConfigPaths: []string{
				"~/.config/doctl",
			},
		},

		// Linode
		{
			ID:       "linode-cli",
			Name:     "Linode CLI",
			Category: "cloud",
			Icon:     "🟢",
			ConfigPaths: []string{
				"~/.config/linode-cli",
			},
		},

		// Vultr
		{
			ID:       "vultr-cli",
			Name:     "Vultr CLI",
			Category: "cloud",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.vultr-cli.yaml",
			},
		},

		// Hetzner
		{
			ID:       "hcloud",
			Name:     "Hetzner Cloud CLI",
			Category: "cloud",
			Icon:     "🔴",
			ConfigPaths: []string{
				"~/.config/hcloud",
			},
		},

		// ========== CI/CD Tools ==========

		// Act (local GitHub Actions)
		{
			ID:       "act-config",
			Name:     "Act",
			Category: "ci",
			Icon:     "🎭",
			ConfigPaths: []string{
				"~/.actrc",
				"~/.config/act",
			},
		},

		// Nektos Act
		{
			ID:       "nektos",
			Name:     "Nektos",
			Category: "ci",
			Icon:     "🐙",
			ConfigPaths: []string{
				"~/.config/nektos",
			},
		},

		// ========== Container Registry ==========

		// Skopeo
		{
			ID:       "skopeo",
			Name:     "Skopeo",
			Category: "container",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/containers",
			},
		},

		// Buildah
		{
			ID:       "buildah",
			Name:     "Buildah",
			Category: "container",
			Icon:     "🏗️",
			ConfigPaths: []string{
				"~/.config/buildah",
			},
		},

		// ========== Code Analysis ==========

		// Ctags
		{
			ID:       "ctags",
			Name:     "Ctags",
			Category: "code",
			Icon:     "🏷️",
			ConfigPaths: []string{
				"~/.ctags",
				"~/.ctags.d",
			},
		},

		// Cscope
		{
			ID:       "cscope",
			Name:     "Cscope",
			Category: "code",
			Icon:     "🔬",
			ConfigPaths: []string{
				"~/.cscoperc",
			},
		},

		// ========== Spell Checking ==========

		// Aspell
		{
			ID:       "aspell",
			Name:     "Aspell",
			Category: "text",
			Icon:     "📖",
			ConfigPaths: []string{
				"~/.aspell.conf",
			},
		},

		// Hunspell
		{
			ID:       "hunspell",
			Name:     "Hunspell",
			Category: "text",
			Icon:     "📚",
			ConfigPaths: []string{
				"~/.hunspell_en_US",
			},
		},

		// ========== Calculator ==========

		// bc
		{
			ID:       "bc",
			Name:     "bc",
			Category: "util",
			Icon:     "🔢",
			ConfigPaths: []string{
				"~/.bcrc",
			},
		},

		// Qalc
		{
			ID:       "qalc",
			Name:     "Qalculate",
			Category: "util",
			Icon:     "🧮",
			ConfigPaths: []string{
				"~/.config/qalculate",
			},
		},

		// ========== Screenshot Tools ==========

		// Flameshot
		{
			ID:       "flameshot-config",
			Name:     "Flameshot",
			Category: "screenshot",
			Icon:     "🔥",
			ConfigPaths: []string{
				"~/.config/flameshot",
			},
		},

		// Shutter
		{
			ID:       "shutter",
			Name:     "Shutter",
			Category: "screenshot",
			Icon:     "📷",
			ConfigPaths: []string{
				"~/.shutter",
			},
		},

		// ========== Screencast ==========

		// Asciinema
		{
			ID:       "asciinema",
			Name:     "Asciinema",
			Category: "screencast",
			Icon:     "🎬",
			ConfigPaths: []string{
				"~/.config/asciinema",
			},
		},

		// Peek
		{
			ID:       "peek",
			Name:     "Peek",
			Category: "screencast",
			Icon:     "👀",
			ConfigPaths: []string{
				"~/.config/peek",
			},
		},

		// ========== PDF Tools ==========

		// pdftk
		{
			ID:       "pdftk",
			Name:     "pdftk",
			Category: "pdf",
			Icon:     "📄",
			ConfigPaths: []string{
				"~/.pdftk",
			},
		},

		// Poppler
		{
			ID:       "poppler",
			Name:     "Poppler",
			Category: "pdf",
			Icon:     "📃",
			ConfigPaths: []string{
				"~/.config/poppler",
			},
		},

		// ========== Archive Tools ==========

		// p7zip
		{
			ID:       "p7zip",
			Name:     "7-Zip",
			Category: "archive",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.7z",
			},
		},

		// Atool
		{
			ID:       "atool",
			Name:     "Atool",
			Category: "archive",
			Icon:     "🗜️",
			ConfigPaths: []string{
				"~/.atoolrc",
			},
		},

		// ========== System Info ==========

		// Screenfetch
		{
			ID:       "screenfetch",
			Name:     "Screenfetch",
			Category: "sysinfo",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.screenfetchrc",
			},
		},

		// Pfetch
		{
			ID:       "pfetch",
			Name:     "Pfetch",
			Category: "sysinfo",
			Icon:     "🐧",
			ConfigPaths: []string{
				"~/.config/pfetch",
			},
		},

		// ========== Network Utilities ==========

		// Netcat
		{
			ID:       "netcat",
			Name:     "Netcat",
			Category: "network",
			Icon:     "🔌",
			ConfigPaths: []string{
				"~/.netcat",
			},
		},

		// Socat
		{
			ID:       "socat",
			Name:     "Socat",
			Category: "network",
			Icon:     "🔗",
			ConfigPaths: []string{
				"~/.socat",
			},
		},

		// ========== Disk Utilities ==========

		// Ncdu
		{
			ID:       "ncdu",
			Name:     "Ncdu",
			Category: "disk",
			Icon:     "💾",
			ConfigPaths: []string{
				"~/.ncdu",
			},
		},

		// Dua
		{
			ID:       "dua",
			Name:     "Dua",
			Category: "disk",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/dua",
			},
		},

		// ========== Process Management ==========

		// Supervisor
		{
			ID:       "supervisor",
			Name:     "Supervisor",
			Category: "process",
			Icon:     "👷",
			ConfigPaths: []string{
				"~/.supervisord.conf",
			},
		},

		// PM2
		{
			ID:       "pm2",
			Name:     "PM2",
			Category: "process",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.pm2",
			},
		},

		// ========== Cron Alternatives ==========

		// Systemd Timers
		{
			ID:       "systemd-user",
			Name:     "Systemd User",
			Category: "scheduler",
			Icon:     "⏰",
			ConfigPaths: []string{
				"~/.config/systemd/user",
			},
		},

		// ========== Language Servers ==========

		// TypeScript LSP
		{
			ID:       "typescript-lsp",
			Name:     "TypeScript LSP",
			Category: "lsp",
			Icon:     "📘",
			ConfigPaths: []string{
				"~/.config/typescript",
			},
		},

		// Python LSP
		{
			ID:       "pylsp",
			Name:     "Python LSP",
			Category: "lsp",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.config/pylsp",
			},
		},

		// Lua LSP
		{
			ID:       "lua-language-server",
			Name:     "Lua Language Server",
			Category: "lsp",
			Icon:     "🌙",
			ConfigPaths: []string{
				"~/.config/lua-language-server",
			},
		},

		// ========== Markdown Tools ==========

		// Pandoc
		{
			ID:       "pandoc",
			Name:     "Pandoc",
			Category: "markdown",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.pandoc",
			},
		},

		// Markdownlint
		{
			ID:       "markdownlint",
			Name:     "Markdownlint",
			Category: "markdown",
			Icon:     "✅",
			ConfigPaths: []string{
				"~/.markdownlint.json",
				"~/.markdownlintrc",
			},
		},

		// ========== Web Browsers ==========

		// Firefox
		{
			ID:       "firefox",
			Name:     "Firefox",
			Category: "browser",
			Icon:     "🦊",
			ConfigPaths: []string{
				"~/.mozilla/firefox",
			},
		},

		// Chromium
		{
			ID:       "chromium",
			Name:     "Chromium",
			Category: "browser",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.config/chromium",
			},
		},

		// Brave
		{
			ID:       "brave",
			Name:     "Brave",
			Category: "browser",
			Icon:     "🦁",
			ConfigPaths: []string{
				"~/.config/BraveSoftware",
			},
		},

		// ========== Input Method ==========

		// Fcitx
		{
			ID:       "fcitx",
			Name:     "Fcitx",
			Category: "input",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/fcitx",
				"~/.config/fcitx5",
			},
		},

		// IBus
		{
			ID:       "ibus",
			Name:     "IBus",
			Category: "input",
			Icon:     "🔤",
			ConfigPaths: []string{
				"~/.config/ibus",
			},
		},

		// ========== X11 Configuration ==========

		// Xmodmap
		{
			ID:       "xmodmap",
			Name:     "Xmodmap",
			Category: "x11",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.Xmodmap",
			},
		},

		// Xinit
		{
			ID:       "xinit",
			Name:     "Xinit",
			Category: "x11",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.xinitrc",
				"~/.xprofile",
			},
		},

		// Xorg
		{
			ID:       "xorg",
			Name:     "Xorg",
			Category: "x11",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/xorg.conf.d",
				"/etc/X11/xorg.conf.d",
			},
		},

		// ========== Additional AI Tools ==========

		// Cursor AI Rules
		{
			ID:       "cursor-rules",
			Name:     "Cursor Rules",
			Category: "ai",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.cursor/rules",
				"~/.cursorrules",
			},
		},

		// Cody
		{
			ID:       "cody",
			Name:     "Sourcegraph Cody",
			Category: "ai",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.config/cody",
			},
		},

		// Tabby
		{
			ID:       "tabby",
			Name:     "Tabby",
			Category: "ai",
			Icon:     "🐱",
			ConfigPaths: []string{
				"~/.tabby",
				"~/.config/tabby",
			},
		},

		// ========== Additional Dev Tools ==========

		// Granted (AWS role switching)
		{
			ID:       "granted",
			Name:     "Granted",
			Category: "cloud",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.granted",
			},
		},

		// Saml2aws
		{
			ID:       "saml2aws",
			Name:     "saml2aws",
			Category: "cloud",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.saml2aws",
			},
		},

		// Chamber
		{
			ID:       "chamber",
			Name:     "Chamber",
			Category: "secrets",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.chamber",
			},
		},

		// ========== Additional CLI Tools ==========

		// Bashrc
		{
			ID:       "bashrc",
			Name:     "Bash RC",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.bashrc",
				"~/.bash_profile",
				"~/.bash_aliases",
				"~/.bash_logout",
			},
		},

		// ========== Additional Editors ==========

		// Micro
		{
			ID:       "micro",
			Name:     "Micro Editor",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/micro",
			},
		},

		// Nano
		{
			ID:       "nano",
			Name:     "Nano",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.nanorc",
				"~/.config/nano",
			},
		},

		// ========== Additional Productivity ==========

		// Finicky (macOS default browser)
		{
			ID:       "finicky",
			Name:     "Finicky",
			Category: "macos",
			Icon:     "🔗",
			ConfigPaths: []string{
				"~/.finicky.js",
			},
		},

		// Rectangle (macOS window manager)
		{
			ID:       "rectangle",
			Name:     "Rectangle",
			Category: "macos",
			Icon:     "🪟",
			ConfigPaths: []string{
				"~/Library/Preferences/com.knollsoft.Rectangle.plist",
			},
		},

		// Hidden Bar
		{
			ID:       "hiddenbar",
			Name:     "Hidden Bar",
			Category: "macos",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/Library/Preferences/com.dwarvesv.minimalbar.plist",
			},
		},

		// ========== Additional System ==========

		// Hosts file
		{
			ID:       "hosts",
			Name:     "Hosts File",
			Category: "system",
			Icon:     "🌐",
			ConfigPaths: []string{
				"/etc/hosts",
			},
		},

		// SSH Config
		{
			ID:       "ssh-config",
			Name:     "SSH Config",
			Category: "system",
			Icon:     "🔑",
			ConfigPaths: []string{
				"~/.ssh/config",
				"~/.ssh/authorized_keys",
			},
		},

		// GPG
		{
			ID:       "gpg-config",
			Name:     "GPG Config",
			Category: "security",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.gnupg/gpg.conf",
				"~/.gnupg/gpg-agent.conf",
			},
		},

		// ========== Additional Popular Tools ==========

		// Superfile
		{
			ID:       "superfile",
			Name:     "Superfile",
			Category: "cli",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/superfile",
			},
		},

		// Yeet
		{
			ID:       "yeet",
			Name:     "Yeet",
			Category: "cli",
			Icon:     "🗑️",
			ConfigPaths: []string{
				"~/.config/yeet",
			},
		},

		// Gitu (TUI git client)
		{
			ID:       "gitu",
			Name:     "Gitu",
			Category: "git",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/gitu",
			},
		},

		// Gitoxide
		{
			ID:       "gitoxide",
			Name:     "Gitoxide",
			Category: "git",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/gitoxide",
			},
		},

		// Zed Editor
		{
			ID:       "zed-editor",
			Name:     "Zed",
			Category: "editor",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/zed",
			},
		},

		// Lapce
		{
			ID:       "lapce",
			Name:     "Lapce",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/lapce",
				"~/.config/lapce-stable",
			},
		},

		// Windsurf
		{
			ID:       "windsurf",
			Name:     "Windsurf",
			Category: "editor",
			Icon:     "🏄",
			ConfigPaths: []string{
				"~/.windsurf",
				"~/.config/Windsurf",
			},
		},

		// Mise (dev environment)
		{
			ID:       "mise-config",
			Name:     "Mise Config",
			Category: "dev",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.config/mise",
				"~/.mise.toml",
			},
		},

		// DevPod
		{
			ID:       "devpod",
			Name:     "DevPod",
			Category: "dev",
			Icon:     "🐳",
			ConfigPaths: []string{
				"~/.devpod",
			},
		},

		// Devbox
		{
			ID:       "devbox",
			Name:     "Devbox",
			Category: "dev",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/devbox",
			},
		},

		// Flox
		{
			ID:       "flox",
			Name:     "Flox",
			Category: "dev",
			Icon:     "❄️",
			ConfigPaths: []string{
				"~/.config/flox",
			},
		},

		// Atac (API client TUI)
		{
			ID:       "atac",
			Name:     "ATAC",
			Category: "api",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.config/atac",
			},
		},

		// Slumber (REST client TUI)
		{
			ID:       "slumber",
			Name:     "Slumber",
			Category: "api",
			Icon:     "😴",
			ConfigPaths: []string{
				"~/.config/slumber",
			},
		},

		// Serpl (Search and Replace TUI)
		{
			ID:       "serpl",
			Name:     "Serpl",
			Category: "cli",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/serpl",
			},
		},

		// Television (Fuzzy Finder)
		{
			ID:       "television",
			Name:     "Television",
			Category: "cli",
			Icon:     "📺",
			ConfigPaths: []string{
				"~/.config/television",
			},
		},

		// Oxker (Docker TUI)
		{
			ID:       "oxker",
			Name:     "Oxker",
			Category: "container",
			Icon:     "🐳",
			ConfigPaths: []string{
				"~/.config/oxker",
			},
		},

		// Orbstack
		{
			ID:       "orbstack",
			Name:     "OrbStack",
			Category: "container",
			Icon:     "🔮",
			ConfigPaths: []string{
				"~/.orbstack",
			},
		},

		// Colima
		{
			ID:       "colima-config",
			Name:     "Colima Config",
			Category: "container",
			Icon:     "🐋",
			ConfigPaths: []string{
				"~/.colima",
			},
		},

		// ========== Modern CLI & TUI Tools ==========

		// Jnv (JSON navigator)
		{
			ID:       "jnv",
			Name:     "jnv",
			Category: "cli",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/jnv",
			},
		},

		// Oatmeal (AI chat TUI)
		{
			ID:       "oatmeal",
			Name:     "Oatmeal",
			Category: "ai",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.config/oatmeal",
			},
		},

		// ========== Database TUI Tools ==========

		// Gobang (Database TUI)
		{
			ID:       "gobang",
			Name:     "Gobang",
			Category: "database",
			Icon:     "🗃️",
			ConfigPaths: []string{
				"~/.config/gobang",
			},
		},

		// Dblab (Database TUI)
		{
			ID:       "dblab",
			Name:     "DBLab",
			Category: "database",
			Icon:     "🗃️",
			ConfigPaths: []string{
				"~/.config/dblab",
			},
		},

		// ========== Security Tools ==========

		// Rustscan
		{
			ID:       "rustscan",
			Name:     "RustScan",
			Category: "security",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/rustscan",
			},
		},

		// Feroxbuster
		{
			ID:       "feroxbuster",
			Name:     "Feroxbuster",
			Category: "security",
			Icon:     "🦀",
			ConfigPaths: []string{
				"~/.config/feroxbuster",
			},
		},

		// ========== Terminal Multiplexers ==========

		// Zellij Layouts
		{
			ID:       "zellij-layouts",
			Name:     "Zellij Layouts",
			Category: "terminal",
			Icon:     "📐",
			ConfigPaths: []string{
				"~/.config/zellij/layouts",
			},
		},

		// ========== Shell Enhancements ==========

		// Hishtory (Shell history)
		{
			ID:       "hishtory",
			Name:     "Hishtory",
			Category: "shell",
			Icon:     "📜",
			ConfigPaths: []string{
				"~/.hishtory",
			},
		},

		// ========== Note Taking ==========

		// Nb (CLI notes)
		{
			ID:       "nb",
			Name:     "nb",
			Category: "notes",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.nbrc",
				"~/.nb",
			},
		},

		// Dn (Daily notes)
		{
			ID:       "dn",
			Name:     "dn",
			Category: "notes",
			Icon:     "📅",
			ConfigPaths: []string{
				"~/.config/dn",
			},
		},

		// ========== System Monitoring ==========

		// Below (System monitor)
		{
			ID:       "below",
			Name:     "Below",
			Category: "monitor",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/below",
				"/etc/below",
			},
		},

		// ========== Additional Tools to reach 750 ==========

		// Amber (Code search)
		{
			ID:       "amber",
			Name:     "Amber",
			Category: "cli",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/amber",
			},
		},

		// Pdu (Parallel disk usage)
		{
			ID:       "pdu",
			Name:     "pdu",
			Category: "disk",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/pdu",
			},
		},

		// Erdtree (File tree)
		{
			ID:       "erdtree",
			Name:     "Erdtree",
			Category: "cli",
			Icon:     "🌳",
			ConfigPaths: []string{
				"~/.config/erdtree",
			},
		},

		// Onefetch (Git repo info)
		{
			ID:       "onefetch",
			Name:     "Onefetch",
			Category: "git",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/onefetch",
			},
		},

		// Macchina (System info)
		{
			ID:       "macchina",
			Name:     "Macchina",
			Category: "system",
			Icon:     "💻",
			ConfigPaths: []string{
				"~/.config/macchina",
			},
		},

		// Rip (rm replacement)
		{
			ID:       "rip",
			Name:     "rip",
			Category: "cli",
			Icon:     "🗑️",
			ConfigPaths: []string{
				"~/.config/rip",
			},
		},

		// ========== More Modern Tools ==========

		// Zrok (Tunneling)
		{
			ID:       "zrok",
			Name:     "zrok",
			Category: "network",
			Icon:     "🚇",
			ConfigPaths: []string{
				"~/.zrok",
			},
		},

		// Ngrok
		{
			ID:       "ngrok",
			Name:     "ngrok",
			Category: "network",
			Icon:     "🚇",
			ConfigPaths: []string{
				"~/.ngrok2",
				"~/.config/ngrok",
			},
		},

		// Bore (TCP tunnel)
		{
			ID:       "bore",
			Name:     "Bore",
			Category: "network",
			Icon:     "🕳️",
			ConfigPaths: []string{
				"~/.config/bore",
			},
		},

		// Miniserve (File server)
		{
			ID:       "miniserve",
			Name:     "Miniserve",
			Category: "server",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/miniserve",
			},
		},

		// Dufs (File server)
		{
			ID:       "dufs",
			Name:     "Dufs",
			Category: "server",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/dufs",
			},
		},

		// Simple-http-server
		{
			ID:       "simple-http-server",
			Name:     "Simple HTTP Server",
			Category: "server",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.config/simple-http-server",
			},
		},

		// ========== Rust CLI Tools ==========

		// Bacon (Rust background checker)
		{
			ID:       "bacon",
			Name:     "Bacon",
			Category: "rust",
			Icon:     "🥓",
			ConfigPaths: []string{
				"~/.config/bacon",
			},
		},

		// Cargo-watch
		{
			ID:       "cargo-watch",
			Name:     "Cargo Watch",
			Category: "rust",
			Icon:     "👀",
			ConfigPaths: []string{
				"~/.config/cargo-watch",
			},
		},

		// ========== JavaScript/Node Tools ==========

		// Corepack
		{
			ID:       "corepack",
			Name:     "Corepack",
			Category: "node",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/corepack",
			},
		},

		// ========== Python Tools ==========

		// Rye (Python package manager)
		{
			ID:       "rye",
			Name:     "Rye",
			Category: "python",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.rye",
			},
		},

		// Pixi (Package manager)
		{
			ID:       "pixi",
			Name:     "Pixi",
			Category: "python",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.pixi",
			},
		},

		// ========== Go Tools ==========

		// Air (Live reload)
		{
			ID:       "air",
			Name:     "Air",
			Category: "go",
			Icon:     "💨",
			ConfigPaths: []string{
				"~/.air.toml",
			},
		},

		// ========== Zig Tools ==========

		// Zigup
		{
			ID:       "zigup",
			Name:     "Zigup",
			Category: "zig",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.config/zigup",
			},
		},

		// ========== Documentation Tools ==========

		// Zola (Static site)
		{
			ID:       "zola",
			Name:     "Zola",
			Category: "docs",
			Icon:     "📄",
			ConfigPaths: []string{
				"~/.config/zola",
			},
		},

		// ========== Image Tools ==========

		// Image (Image viewer)
		{
			ID:       "viu",
			Name:     "viu",
			Category: "image",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.config/viu",
			},
		},

		// Chafa (Image to ASCII)
		{
			ID:       "chafa",
			Name:     "Chafa",
			Category: "image",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.config/chafa",
			},
		},

		// ========== Additional Modern Tools ==========

		// Hwatch (Modern watch)
		{
			ID:       "hwatch",
			Name:     "hwatch",
			Category: "cli",
			Icon:     "👀",
			ConfigPaths: []string{
				"~/.config/hwatch",
			},
		},

		// Xcp (Extended cp)
		{
			ID:       "xcp",
			Name:     "xcp",
			Category: "cli",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/xcp",
			},
		},

		// Nushell Plugins
		{
			ID:       "nushell-plugins",
			Name:     "Nushell Plugins",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.config/nushell/plugins",
			},
		},

		// Fig (Terminal autocomplete)
		{
			ID:       "fig",
			Name:     "Fig",
			Category: "terminal",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.fig",
			},
		},

		// Iterm2 Shell Integration
		{
			ID:       "iterm2-shell",
			Name:     "iTerm2 Shell Integration",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.iterm2_shell_integration.zsh",
				"~/.iterm2_shell_integration.bash",
			},
		},

		// Ghostty Themes
		{
			ID:       "ghostty-themes",
			Name:     "Ghostty Themes",
			Category: "terminal",
			Icon:     "👻",
			ConfigPaths: []string{
				"~/.config/ghostty/themes",
			},
		},

		// Zsh Plugins
		{
			ID:       "zsh-plugins",
			Name:     "Zsh Plugins",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.zsh/plugins",
				"~/.local/share/zsh/plugins",
			},
		},

		// Zinit
		{
			ID:       "zinit",
			Name:     "Zinit",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.zinit",
			},
		},

		// Antidote
		{
			ID:       "antidote",
			Name:     "Antidote",
			Category: "shell",
			Icon:     "💊",
			ConfigPaths: []string{
				"~/.antidote",
				"~/.zsh_plugins.txt",
			},
		},

		// Zap (Zsh plugin manager)
		{
			ID:       "zap",
			Name:     "Zap",
			Category: "shell",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.local/share/zap",
			},
		},

		// ========== Additional Apps to 800+ ==========

		// Mask (Task runner)
		{
			ID:       "mask",
			Name:     "Mask",
			Category: "dev",
			Icon:     "🎭",
			ConfigPaths: []string{
				"~/.config/mask",
			},
		},

		// Mprocs (Multi-process runner)
		{
			ID:       "mprocs",
			Name:     "mprocs",
			Category: "dev",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/mprocs",
			},
		},

		// Zoxide data
		{
			ID:       "zoxide-data",
			Name:     "Zoxide Data",
			Category: "shell",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.local/share/zoxide",
			},
		},

		// Atuin data
		{
			ID:       "atuin-data",
			Name:     "Atuin Data",
			Category: "shell",
			Icon:     "📜",
			ConfigPaths: []string{
				"~/.local/share/atuin",
			},
		},

		// Fish functions
		{
			ID:       "fish-functions",
			Name:     "Fish Functions",
			Category: "shell",
			Icon:     "🐟",
			ConfigPaths: []string{
				"~/.config/fish/functions",
			},
		},

		// Fish completions
		{
			ID:       "fish-completions",
			Name:     "Fish Completions",
			Category: "shell",
			Icon:     "🐟",
			ConfigPaths: []string{
				"~/.config/fish/completions",
			},
		},

		// Neovim Lazy
		{
			ID:       "nvim-lazy",
			Name:     "Neovim Lazy",
			Category: "editor",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.local/share/nvim/lazy",
			},
		},

		// Neovim Mason
		{
			ID:       "nvim-mason",
			Name:     "Neovim Mason",
			Category: "editor",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.local/share/nvim/mason",
			},
		},

		// Tmux plugins
		{
			ID:       "tmux-plugins",
			Name:     "Tmux Plugins",
			Category: "terminal",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.tmux/plugins",
			},
		},

		// TPM (Tmux Plugin Manager)
		{
			ID:       "tpm",
			Name:     "TPM",
			Category: "terminal",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.tmux/plugins/tpm",
			},
		},

		// Aerospace layouts
		{
			ID:       "aerospace-layouts",
			Name:     "AeroSpace Layouts",
			Category: "macos",
			Icon:     "🪟",
			ConfigPaths: []string{
				"~/.config/aerospace/layouts",
			},
		},

		// Yabai scripts
		{
			ID:       "yabai-scripts",
			Name:     "Yabai Scripts",
			Category: "macos",
			Icon:     "🪟",
			ConfigPaths: []string{
				"~/.config/yabai/scripts",
			},
		},

		// Sketchybar plugins
		{
			ID:       "sketchybar-plugins",
			Name:     "SketchyBar Plugins",
			Category: "macos",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/sketchybar/plugins",
			},
		},

		// Raycast scripts
		{
			ID:       "raycast-scripts",
			Name:     "Raycast Scripts",
			Category: "macos",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/raycast/scripts",
			},
		},

		// Hammerspoon Spoons
		{
			ID:       "hammerspoon-spoons",
			Name:     "Hammerspoon Spoons",
			Category: "macos",
			Icon:     "🔨",
			ConfigPaths: []string{
				"~/.hammerspoon/Spoons",
			},
		},

		// Zsh completions
		{
			ID:       "zsh-completions",
			Name:     "Zsh Completions",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.zsh/completions",
				"~/.local/share/zsh/completions",
			},
		},

		// Zsh functions
		{
			ID:       "zsh-functions",
			Name:     "Zsh Functions",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.zsh/functions",
				"~/.local/share/zsh/functions",
			},
		},

		// Bash completions
		{
			ID:       "bash-completions",
			Name:     "Bash Completions",
			Category: "shell",
			Icon:     "🐚",
			ConfigPaths: []string{
				"~/.bash_completion.d",
				"~/.local/share/bash-completion",
			},
		},

		// Tmuxinator
		{
			ID:       "tmuxinator",
			Name:     "Tmuxinator",
			Category: "terminal",
			Icon:     "📺",
			ConfigPaths: []string{
				"~/.tmuxinator",
				"~/.config/tmuxinator",
			},
		},

		// Smug (Tmux session manager)
		{
			ID:       "smug",
			Name:     "Smug",
			Category: "terminal",
			Icon:     "📺",
			ConfigPaths: []string{
				"~/.config/smug",
			},
		},

		// Tmuxp
		{
			ID:       "tmuxp",
			Name:     "Tmuxp",
			Category: "terminal",
			Icon:     "📺",
			ConfigPaths: []string{
				"~/.tmuxp",
				"~/.config/tmuxp",
			},
		},

		// Kakoune
		{
			ID:       "kakoune",
			Name:     "Kakoune",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/kak",
				"~/.config/kakoune",
			},
		},

		// Tilde (CLI editor)
		{
			ID:       "tilde",
			Name:     "Tilde",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/tilde",
			},
		},

		// Amp
		{
			ID:       "amp",
			Name:     "Amp",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/amp",
				"~/.amp",
			},
		},

		// Ox Editor
		{
			ID:       "ox",
			Name:     "Ox Editor",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/ox",
				"~/.ox.ron",
			},
		},

		// Zee Editor
		{
			ID:       "zee",
			Name:     "Zee",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/zee",
			},
		},

		// Smith Editor
		{
			ID:       "smith",
			Name:     "Smith",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/smith",
			},
		},

		// Kibi Editor
		{
			ID:       "kibi",
			Name:     "Kibi",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/kibi",
			},
		},

		// Micro Editor
		{
			ID:       "micro",
			Name:     "Micro",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/micro",
			},
		},

		// Kilo Editor
		{
			ID:       "kilo",
			Name:     "Kilo",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/kilo",
			},
		},

		// Dte Editor
		{
			ID:       "dte",
			Name:     "Dte",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/dte",
				"~/.dte",
			},
		},

		// Vis Editor
		{
			ID:       "vis",
			Name:     "Vis",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/vis",
			},
		},

		// Mle Editor
		{
			ID:       "mle",
			Name:     "Mle",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/mle",
				"~/.mlerc",
			},
		},

		// Vy Editor
		{
			ID:       "vy",
			Name:     "Vy",
			Category: "editor",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.vyrc",
			},
		},

		// Tig (Text-mode interface for Git)
		{
			ID:       "tig",
			Name:     "Tig",
			Category: "git",
			Icon:     "🔀",
			ConfigPaths: []string{
				"~/.tigrc",
				"~/.config/tig",
			},
		},

		// Difftastic
		{
			ID:       "difftastic",
			Name:     "Difftastic",
			Category: "git",
			Icon:     "🔀",
			ConfigPaths: []string{
				"~/.config/difft",
			},
		},

		// Gitu
		{
			ID:       "gitu",
			Name:     "Gitu",
			Category: "git",
			Icon:     "🔀",
			ConfigPaths: []string{
				"~/.config/gitu",
			},
		},

		// Serie (Rich git commit graph)
		{
			ID:       "serie",
			Name:     "Serie",
			Category: "git",
			Icon:     "🔀",
			ConfigPaths: []string{
				"~/.config/serie",
			},
		},

		// Gitwatch
		{
			ID:       "gitwatch",
			Name:     "Gitwatch",
			Category: "git",
			Icon:     "🔀",
			ConfigPaths: []string{
				"~/.config/gitwatch",
			},
		},

		// Onefetch
		{
			ID:       "onefetch",
			Name:     "Onefetch",
			Category: "cli",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/onefetch",
			},
		},

		// Tokei (Code statistics)
		{
			ID:       "tokei",
			Name:     "Tokei",
			Category: "cli",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/tokei",
				"~/.tokeignore",
			},
		},

		// Hyperfine (Benchmarking)
		{
			ID:       "hyperfine",
			Name:     "Hyperfine",
			Category: "cli",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/hyperfine",
			},
		},

		// Watchexec
		{
			ID:       "watchexec",
			Name:     "Watchexec",
			Category: "cli",
			Icon:     "⌨️",
			ConfigPaths: []string{
				"~/.config/watchexec",
			},
		},

		// Just (Command runner)
		{
			ID:       "just",
			Name:     "Just",
			Category: "dev",
			Icon:     "🛠️",
			ConfigPaths: []string{
				"~/.config/just",
				"~/.justfile",
			},
		},

		// Direnv
		{
			ID:       "direnv",
			Name:     "Direnv",
			Category: "dev",
			Icon:     "🛠️",
			ConfigPaths: []string{
				"~/.config/direnv",
				"~/.direnvrc",
			},
		},

		// ========== More CLI Tools ==========

		// Bat (cat with syntax highlighting)
		{
			ID:       "bat",
			Name:     "Bat",
			Category: "cli",
			Icon:     "🦇",
			ConfigPaths: []string{
				"~/.config/bat",
			},
		},

		// Fd (fast find alternative)
		{
			ID:       "fd",
			Name:     "Fd",
			Category: "cli",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.config/fd",
				"~/.fdignore",
			},
		},

		// Procs (ps replacement)
		{
			ID:       "procs",
			Name:     "Procs",
			Category: "cli",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/procs",
			},
		},

		// Dust (du replacement)
		{
			ID:       "dust",
			Name:     "Dust",
			Category: "cli",
			Icon:     "💨",
			ConfigPaths: []string{
				"~/.config/dust",
			},
		},

		// Bottom (htop replacement)
		{
			ID:       "bottom",
			Name:     "Bottom",
			Category: "cli",
			Icon:     "📈",
			ConfigPaths: []string{
				"~/.config/bottom",
			},
		},

		// Broot (tree replacement)
		{
			ID:       "broot",
			Name:     "Broot",
			Category: "cli",
			Icon:     "🌳",
			ConfigPaths: []string{
				"~/.config/broot",
			},
		},

		// Zoxide (cd replacement)
		{
			ID:       "zoxide",
			Name:     "Zoxide",
			Category: "cli",
			Icon:     "📂",
			ConfigPaths: []string{
				"~/.config/zoxide",
			},
		},

		// Atuin (shell history)
		{
			ID:       "atuin",
			Name:     "Atuin",
			Category: "shell",
			Icon:     "📜",
			ConfigPaths: []string{
				"~/.config/atuin",
			},
		},

		// McFly (shell history)
		{
			ID:       "mcfly",
			Name:     "McFly",
			Category: "shell",
			Icon:     "🔙",
			ConfigPaths: []string{
				"~/.config/mcfly",
			},
		},

		// Navi (cheatsheet tool)
		{
			ID:       "navi",
			Name:     "Navi",
			Category: "cli",
			Icon:     "📖",
			ConfigPaths: []string{
				"~/.config/navi",
			},
		},

		// Tealdeer (tldr client)
		{
			ID:       "tealdeer",
			Name:     "Tealdeer",
			Category: "cli",
			Icon:     "📚",
			ConfigPaths: []string{
				"~/.config/tealdeer",
			},
		},

		// Glow (markdown reader)
		{
			ID:       "glow",
			Name:     "Glow",
			Category: "cli",
			Icon:     "✨",
			ConfigPaths: []string{
				"~/.config/glow",
			},
		},

		// Charm (CLI tools)
		{
			ID:       "charm",
			Name:     "Charm",
			Category: "cli",
			Icon:     "💫",
			ConfigPaths: []string{
				"~/.config/charm",
			},
		},

		// Pueue (task manager)
		{
			ID:       "pueue",
			Name:     "Pueue",
			Category: "cli",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/.config/pueue",
			},
		},

		// Taskwarrior
		{
			ID:       "taskwarrior",
			Name:     "Taskwarrior",
			Category: "productivity",
			Icon:     "✅",
			ConfigPaths: []string{
				"~/.config/task",
				"~/.taskrc",
			},
		},

		// Timewarrior
		{
			ID:       "timewarrior",
			Name:     "Timewarrior",
			Category: "productivity",
			Icon:     "⏱️",
			ConfigPaths: []string{
				"~/.config/timewarrior",
				"~/.timewarrior",
			},
		},

		// Todoman
		{
			ID:       "todoman",
			Name:     "Todoman",
			Category: "productivity",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/todoman",
			},
		},

		// Vdirsyncer
		{
			ID:       "vdirsyncer",
			Name:     "Vdirsyncer",
			Category: "productivity",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.config/vdirsyncer",
			},
		},

		// Tuir (terminal reddit)
		{
			ID:       "tuir",
			Name:     "Tuir",
			Category: "social",
			Icon:     "👽",
			ConfigPaths: []string{
				"~/.config/tuir",
			},
		},

		// Newsboat
		{
			ID:       "newsboat",
			Name:     "Newsboat",
			Category: "news",
			Icon:     "📰",
			ConfigPaths: []string{
				"~/.config/newsboat",
				"~/.newsboat",
			},
		},

		// Castero (podcast client)
		{
			ID:       "castero",
			Name:     "Castero",
			Category: "media",
			Icon:     "🎙️",
			ConfigPaths: []string{
				"~/.config/castero",
			},
		},

		// Spotifyd
		{
			ID:       "spotifyd",
			Name:     "Spotifyd",
			Category: "media",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.config/spotifyd",
			},
		},

		// Spotify-tui
		{
			ID:       "spotify-tui",
			Name:     "Spotify TUI",
			Category: "media",
			Icon:     "🎧",
			ConfigPaths: []string{
				"~/.config/spotify-tui",
			},
		},

		// Ncspot
		{
			ID:       "ncspot",
			Name:     "Ncspot",
			Category: "media",
			Icon:     "🎼",
			ConfigPaths: []string{
				"~/.config/ncspot",
			},
		},

		// Termusic
		{
			ID:       "termusic",
			Name:     "Termusic",
			Category: "media",
			Icon:     "🎶",
			ConfigPaths: []string{
				"~/.config/termusic",
			},
		},

		// Musikcube
		{
			ID:       "musikcube",
			Name:     "Musikcube",
			Category: "media",
			Icon:     "🎹",
			ConfigPaths: []string{
				"~/.config/musikcube",
			},
		},

		// ========== AI & LLM Tools ==========

		// Ollama
		{
			ID:       "ollama",
			Name:     "Ollama",
			Category: "ai",
			Icon:     "🦙",
			ConfigPaths: []string{
				"~/.ollama",
			},
		},

		// LM Studio
		{
			ID:       "lm-studio",
			Name:     "LM Studio",
			Category: "ai",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.cache/lm-studio",
			},
		},

		// LocalAI
		{
			ID:       "localai",
			Name:     "LocalAI",
			Category: "ai",
			Icon:     "🧠",
			ConfigPaths: []string{
				"~/.config/localai",
			},
		},

		// Aider (AI pair programming)
		{
			ID:       "aider",
			Name:     "Aider",
			Category: "ai",
			Icon:     "🤝",
			ConfigPaths: []string{
				"~/.aider.conf.yml",
				"~/.config/aider",
			},
		},

		// Continue.dev
		{
			ID:       "continue",
			Name:     "Continue",
			Category: "ai",
			Icon:     "➡️",
			ConfigPaths: []string{
				"~/.continue",
			},
		},

		// ========== Modern Terminal Tools ==========

		// Warp Terminal
		{
			ID:       "warp",
			Name:     "Warp",
			Category: "terminal",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.warp",
			},
		},

		// Rio Terminal
		{
			ID:       "rio",
			Name:     "Rio",
			Category: "terminal",
			Icon:     "🌊",
			ConfigPaths: []string{
				"~/.config/rio",
			},
		},

		// Tabby Terminal
		{
			ID:       "tabby",
			Name:     "Tabby Terminal",
			Category: "terminal",
			Icon:     "🐱",
			ConfigPaths: []string{
				"~/.config/tabby",
			},
		},

		// Wave Terminal
		{
			ID:       "wave",
			Name:     "Wave",
			Category: "terminal",
			Icon:     "🌊",
			ConfigPaths: []string{
				"~/.waveterm",
			},
		},

		// ========== Container & Cloud Native ==========

		// Kind
		{
			ID:       "kind",
			Name:     "Kind",
			Category: "kubernetes",
			Icon:     "☸️",
			ConfigPaths: []string{
				"~/.kind",
			},
		},

		// K3d
		{
			ID:       "k3d",
			Name:     "K3d",
			Category: "kubernetes",
			Icon:     "🎯",
			ConfigPaths: []string{
				"~/.k3d",
			},
		},

		// Minikube
		{
			ID:       "minikube",
			Name:     "Minikube",
			Category: "kubernetes",
			Icon:     "🚗",
			ConfigPaths: []string{
				"~/.minikube",
			},
		},

		// Colima (Docker on macOS)
		{
			ID:       "colima",
			Name:     "Colima",
			Category: "container",
			Icon:     "🐳",
			ConfigPaths: []string{
				"~/.colima",
			},
		},

		// OrbStack
		{
			ID:       "orbstack",
			Name:     "OrbStack",
			Category: "container",
			Icon:     "🪐",
			ConfigPaths: []string{
				"~/.orbstack",
			},
		},

		// ========== Database Tools ==========

		// DBeaver
		{
			ID:       "dbeaver",
			Name:     "DBeaver",
			Category: "database",
			Icon:     "🦫",
			ConfigPaths: []string{
				"~/.dbeaver",
			},
		},

		// TablePlus
		{
			ID:       "tableplus",
			Name:     "TablePlus",
			Category: "database",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/Library/Application Support/com.tinyapp.TablePlus",
			},
		},

		// usql
		{
			ID:       "usql",
			Name:     "usql",
			Category: "database",
			Icon:     "🗃️",
			ConfigPaths: []string{
				"~/.config/usql",
				"~/.usqlrc",
			},
		},

		// ========== Security Tools ==========

		// Age (encryption)
		{
			ID:       "age",
			Name:     "Age",
			Category: "security",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.config/age",
			},
		},

		// SOPS
		{
			ID:       "sops",
			Name:     "SOPS",
			Category: "security",
			Icon:     "🔒",
			ConfigPaths: []string{
				"~/.config/sops",
				"~/.sops.yaml",
			},
		},

		// Chezmoi
		{
			ID:       "chezmoi",
			Name:     "Chezmoi",
			Category: "dotfiles",
			Icon:     "🏠",
			ConfigPaths: []string{
				"~/.config/chezmoi",
			},
		},

		// ========== Note Taking ==========

		// Obsidian
		{
			ID:       "obsidian",
			Name:     "Obsidian",
			Category: "notes",
			Icon:     "💎",
			ConfigPaths: []string{
				"~/.config/obsidian",
			},
		},

		// Logseq
		{
			ID:       "logseq",
			Name:     "Logseq",
			Category: "notes",
			Icon:     "📓",
			ConfigPaths: []string{
				"~/.logseq",
			},
		},

		// Zettlr
		{
			ID:       "zettlr",
			Name:     "Zettlr",
			Category: "notes",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/.config/Zettlr",
			},
		},

		// ========== API Development ==========

		// Insomnia
		{
			ID:       "insomnia",
			Name:     "Insomnia",
			Category: "api",
			Icon:     "😴",
			ConfigPaths: []string{
				"~/.config/Insomnia",
			},
		},

		// HTTPie
		{
			ID:       "httpie",
			Name:     "HTTPie",
			Category: "api",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.config/httpie",
			},
		},

		// Posting (TUI HTTP client)
		{
			ID:       "posting",
			Name:     "Posting",
			Category: "api",
			Icon:     "📮",
			ConfigPaths: []string{
				"~/.config/posting",
			},
		},

		// ========== Development Utilities ==========

		// Mise (asdf replacement)
		{
			ID:       "mise",
			Name:     "Mise",
			Category: "dev",
			Icon:     "🔧",
			ConfigPaths: []string{
				"~/.config/mise",
				"~/.mise.toml",
			},
		},

		// Proto (version manager)
		{
			ID:       "proto",
			Name:     "Proto",
			Category: "dev",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.proto",
			},
		},

		// Pixi (conda alternative)
		{
			ID:       "pixi",
			Name:     "Pixi",
			Category: "dev",
			Icon:     "🐍",
			ConfigPaths: []string{
				"~/.pixi",
			},
		},

		// UV (Python package manager)
		{
			ID:       "uv",
			Name:     "UV",
			Category: "python",
			Icon:     "☀️",
			ConfigPaths: []string{
				"~/.config/uv",
			},
		},

		// Rye (Python project manager)
		{
			ID:       "rye",
			Name:     "Rye",
			Category: "python",
			Icon:     "🌾",
			ConfigPaths: []string{
				"~/.rye",
			},
		},

		// Bun
		{
			ID:       "bun",
			Name:     "Bun",
			Category: "javascript",
			Icon:     "🥟",
			ConfigPaths: []string{
				"~/.bun",
				"~/.bunfig.toml",
			},
		},

		// Deno
		{
			ID:       "deno",
			Name:     "Deno",
			Category: "javascript",
			Icon:     "🦕",
			ConfigPaths: []string{
				"~/.deno",
			},
		},

		// ========== 2025 Modern Tools ==========

		// Claude Desktop
		{
			ID:       "claude-desktop",
			Name:     "Claude Desktop",
			Category: "ai",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/Library/Application Support/Claude",
				"~/.config/Claude",
			},
		},

		// Cursor AI
		{
			ID:       "cursor",
			Name:     "Cursor",
			Category: "editor",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.cursor",
				"~/Library/Application Support/Cursor",
				"~/.config/Cursor",
			},
		},

		// Windsurf
		{
			ID:       "windsurf",
			Name:     "Windsurf",
			Category: "editor",
			Icon:     "🏄",
			ConfigPaths: []string{
				"~/.windsurf",
				"~/Library/Application Support/Windsurf",
			},
		},

		// Cline
		{
			ID:       "cline",
			Name:     "Cline",
			Category: "ai",
			Icon:     "🔮",
			ConfigPaths: []string{
				"~/.cline",
				"~/.config/cline",
			},
		},

		// Codestral / Mistral
		{
			ID:       "mistral",
			Name:     "Mistral CLI",
			Category: "ai",
			Icon:     "🌀",
			ConfigPaths: []string{
				"~/.mistral",
				"~/.config/mistral",
			},
		},

		// OpenRouter
		{
			ID:       "openrouter",
			Name:     "OpenRouter",
			Category: "ai",
			Icon:     "🔗",
			ConfigPaths: []string{
				"~/.config/openrouter",
			},
		},

		// Groq
		{
			ID:       "groq",
			Name:     "Groq CLI",
			Category: "ai",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.groq",
				"~/.config/groq",
			},
		},

		// Perplexity
		{
			ID:       "perplexity",
			Name:     "Perplexity CLI",
			Category: "ai",
			Icon:     "🔍",
			ConfigPaths: []string{
				"~/.perplexity",
				"~/.config/perplexity",
			},
		},

		// GitHub CLI Extensions
		{
			ID:       "gh-extensions",
			Name:     "GitHub CLI Extensions",
			Category: "git",
			Icon:     "🔌",
			ConfigPaths: []string{
				"~/.local/share/gh/extensions",
			},
		},

		// GitButler
		{
			ID:       "gitbutler",
			Name:     "GitButler",
			Category: "git",
			Icon:     "🧈",
			ConfigPaths: []string{
				"~/.gitbutler",
				"~/Library/Application Support/GitButler",
			},
		},

		// Fork Git Client
		{
			ID:       "fork",
			Name:     "Fork",
			Category: "git",
			Icon:     "🍴",
			ConfigPaths: []string{
				"~/Library/Application Support/Fork",
			},
		},

		// GitKraken
		{
			ID:       "gitkraken",
			Name:     "GitKraken",
			Category: "git",
			Icon:     "🐙",
			ConfigPaths: []string{
				"~/.gitkraken",
			},
		},

		// Sourcetree
		{
			ID:       "sourcetree",
			Name:     "Sourcetree",
			Category: "git",
			Icon:     "🌳",
			ConfigPaths: []string{
				"~/Library/Application Support/SourceTree",
			},
		},

		// Raycast Extensions
		{
			ID:       "raycast-extensions",
			Name:     "Raycast Extensions",
			Category: "productivity",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/raycast",
				"~/Library/Application Support/Raycast/Extensions",
			},
		},

		// Arc Browser
		{
			ID:       "arc",
			Name:     "Arc Browser",
			Category: "browser",
			Icon:     "🌈",
			ConfigPaths: []string{
				"~/Library/Application Support/Arc",
			},
		},

		// Zen Browser
		{
			ID:       "zen-browser",
			Name:     "Zen Browser",
			Category: "browser",
			Icon:     "🧘",
			ConfigPaths: []string{
				"~/.zen",
				"~/.config/zen-browser",
			},
		},

		// Floorp
		{
			ID:       "floorp",
			Name:     "Floorp",
			Category: "browser",
			Icon:     "🦊",
			ConfigPaths: []string{
				"~/.floorp",
				"~/.config/floorp",
			},
		},

		// Vivaldi
		{
			ID:       "vivaldi",
			Name:     "Vivaldi",
			Category: "browser",
			Icon:     "🎵",
			ConfigPaths: []string{
				"~/.config/vivaldi",
				"~/Library/Application Support/Vivaldi",
			},
		},

		// Brave
		{
			ID:       "brave",
			Name:     "Brave Browser",
			Category: "browser",
			Icon:     "🦁",
			ConfigPaths: []string{
				"~/.config/BraveSoftware",
				"~/Library/Application Support/BraveSoftware",
			},
		},

		// Wezterm
		{
			ID:       "wezterm",
			Name:     "WezTerm",
			Category: "terminal",
			Icon:     "🖥️",
			ConfigPaths: []string{
				"~/.config/wezterm",
				"~/.wezterm.lua",
			},
		},

		// Alacritty
		{
			ID:       "alacritty",
			Name:     "Alacritty",
			Category: "terminal",
			Icon:     "🔺",
			ConfigPaths: []string{
				"~/.config/alacritty",
				"~/.alacritty.toml",
				"~/.alacritty.yml",
			},
		},

		// Foot
		{
			ID:       "foot",
			Name:     "Foot",
			Category: "terminal",
			Icon:     "👣",
			ConfigPaths: []string{
				"~/.config/foot",
			},
		},

		// Contour
		{
			ID:       "contour",
			Name:     "Contour",
			Category: "terminal",
			Icon:     "📺",
			ConfigPaths: []string{
				"~/.config/contour",
			},
		},

		// Superfile
		{
			ID:       "superfile",
			Name:     "Superfile",
			Category: "cli",
			Icon:     "📁",
			ConfigPaths: []string{
				"~/.config/superfile",
			},
		},

		// Joshuto
		{
			ID:       "joshuto",
			Name:     "Joshuto",
			Category: "cli",
			Icon:     "📂",
			ConfigPaths: []string{
				"~/.config/joshuto",
			},
		},

		// Yazi Plugins
		{
			ID:       "yazi-plugins",
			Name:     "Yazi Plugins",
			Category: "cli",
			Icon:     "🔌",
			ConfigPaths: []string{
				"~/.config/yazi/plugins",
			},
		},

		// Zellij Layouts
		{
			ID:       "zellij-layouts",
			Name:     "Zellij Layouts",
			Category: "terminal",
			Icon:     "📐",
			ConfigPaths: []string{
				"~/.config/zellij/layouts",
			},
		},

		// Aerospace
		{
			ID:       "aerospace",
			Name:     "AeroSpace",
			Category: "productivity",
			Icon:     "✈️",
			ConfigPaths: []string{
				"~/.aerospace.toml",
				"~/.config/aerospace",
			},
		},

		// Borders (JankyBorders replacement)
		{
			ID:       "borders",
			Name:     "Borders",
			Category: "productivity",
			Icon:     "🔲",
			ConfigPaths: []string{
				"~/.config/borders",
			},
		},

		// Sketchybar Plugins
		{
			ID:       "sketchybar-plugins",
			Name:     "Sketchybar Plugins",
			Category: "productivity",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/sketchybar/plugins",
			},
		},

		// Ice (menu bar management)
		{
			ID:       "ice",
			Name:     "Ice",
			Category: "productivity",
			Icon:     "🧊",
			ConfigPaths: []string{
				"~/Library/Application Support/Ice",
			},
		},

		// Bartender
		{
			ID:       "bartender",
			Name:     "Bartender",
			Category: "productivity",
			Icon:     "🍺",
			ConfigPaths: []string{
				"~/Library/Application Support/Bartender",
			},
		},

		// Hidden Bar
		{
			ID:       "hiddenbar",
			Name:     "Hidden Bar",
			Category: "productivity",
			Icon:     "👁️",
			ConfigPaths: []string{
				"~/Library/Application Support/Hidden Bar",
			},
		},

		// Linear
		{
			ID:       "linear",
			Name:     "Linear",
			Category: "productivity",
			Icon:     "📋",
			ConfigPaths: []string{
				"~/Library/Application Support/Linear",
			},
		},

		// Notion
		{
			ID:       "notion",
			Name:     "Notion",
			Category: "productivity",
			Icon:     "📓",
			ConfigPaths: []string{
				"~/Library/Application Support/Notion",
			},
		},

		// Craft
		{
			ID:       "craft",
			Name:     "Craft",
			Category: "productivity",
			Icon:     "✍️",
			ConfigPaths: []string{
				"~/Library/Application Support/io.craft.app",
			},
		},

		// Anytype
		{
			ID:       "anytype",
			Name:     "Anytype",
			Category: "productivity",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/Library/Application Support/anytype",
			},
		},

		// Nota (markdown notes)
		{
			ID:       "nota",
			Name:     "Nota",
			Category: "productivity",
			Icon:     "📝",
			ConfigPaths: []string{
				"~/Library/Application Support/Nota",
			},
		},

		// DevPod
		{
			ID:       "devpod",
			Name:     "DevPod",
			Category: "dev",
			Icon:     "🛠️",
			ConfigPaths: []string{
				"~/.devpod",
			},
		},

		// Devbox
		{
			ID:       "devbox",
			Name:     "Devbox",
			Category: "dev",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/devbox",
			},
		},

		// Devenv
		{
			ID:       "devenv",
			Name:     "devenv",
			Category: "dev",
			Icon:     "🌍",
			ConfigPaths: []string{
				"~/.config/devenv",
			},
		},

		// Pixi
		{
			ID:       "pixi",
			Name:     "Pixi",
			Category: "dev",
			Icon:     "🎮",
			ConfigPaths: []string{
				"~/.pixi",
			},
		},

		// Rye
		{
			ID:       "rye",
			Name:     "Rye",
			Category: "python",
			Icon:     "🌾",
			ConfigPaths: []string{
				"~/.rye",
			},
		},

		// UV Python
		{
			ID:       "uv",
			Name:     "UV",
			Category: "python",
			Icon:     "☀️",
			ConfigPaths: []string{
				"~/.uv",
				"~/.config/uv",
			},
		},

		// Podman Desktop
		{
			ID:       "podman-desktop",
			Name:     "Podman Desktop",
			Category: "container",
			Icon:     "🐳",
			ConfigPaths: []string{
				"~/.config/podman-desktop",
			},
		},

		// OrbStack
		{
			ID:       "orbstack",
			Name:     "OrbStack",
			Category: "container",
			Icon:     "🔮",
			ConfigPaths: []string{
				"~/.orbstack",
			},
		},

		// Rancher Desktop
		{
			ID:       "rancher-desktop",
			Name:     "Rancher Desktop",
			Category: "container",
			Icon:     "🐮",
			ConfigPaths: []string{
				"~/.config/rancher-desktop",
				"~/Library/Application Support/rancher-desktop",
			},
		},

		// Teleport
		{
			ID:       "teleport",
			Name:     "Teleport",
			Category: "security",
			Icon:     "🔒",
			ConfigPaths: []string{
				"~/.tsh",
			},
		},

		// Tailscale
		{
			ID:       "tailscale",
			Name:     "Tailscale",
			Category: "network",
			Icon:     "🔗",
			ConfigPaths: []string{
				"~/.config/tailscale",
			},
		},

		// ZeroTier
		{
			ID:       "zerotier",
			Name:     "ZeroTier",
			Category: "network",
			Icon:     "0️⃣",
			ConfigPaths: []string{
				"/var/lib/zerotier-one",
			},
		},

		// Cloudflare WARP
		{
			ID:       "warp",
			Name:     "Cloudflare WARP",
			Category: "network",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.config/warp",
			},
		},

		// Granted (AWS role switching)
		{
			ID:       "granted",
			Name:     "Granted",
			Category: "cloud",
			Icon:     "🔑",
			ConfigPaths: []string{
				"~/.granted",
			},
		},

		// Leapp (cloud access)
		{
			ID:       "leapp",
			Name:     "Leapp",
			Category: "cloud",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.Leapp",
			},
		},

		// Infisical (secrets)
		{
			ID:       "infisical",
			Name:     "Infisical",
			Category: "security",
			Icon:     "🔐",
			ConfigPaths: []string{
				"~/.infisical",
			},
		},

		// Doppler
		{
			ID:       "doppler",
			Name:     "Doppler",
			Category: "security",
			Icon:     "🔑",
			ConfigPaths: []string{
				"~/.doppler",
			},
		},

		// Supabase CLI
		{
			ID:       "supabase",
			Name:     "Supabase CLI",
			Category: "database",
			Icon:     "⚡",
			ConfigPaths: []string{
				"~/.supabase",
			},
		},

		// PlanetScale CLI
		{
			ID:       "planetscale",
			Name:     "PlanetScale",
			Category: "database",
			Icon:     "🪐",
			ConfigPaths: []string{
				"~/.config/planetscale",
			},
		},

		// Turso
		{
			ID:       "turso",
			Name:     "Turso",
			Category: "database",
			Icon:     "🐢",
			ConfigPaths: []string{
				"~/.turso",
			},
		},

		// Neon
		{
			ID:       "neon",
			Name:     "Neon",
			Category: "database",
			Icon:     "💡",
			ConfigPaths: []string{
				"~/.neon",
			},
		},

		// Drizzle
		{
			ID:       "drizzle",
			Name:     "Drizzle",
			Category: "database",
			Icon:     "💧",
			ConfigPaths: []string{
				"~/.config/drizzle",
			},
		},

		// Prisma
		{
			ID:       "prisma",
			Name:     "Prisma",
			Category: "database",
			Icon:     "🔷",
			ConfigPaths: []string{
				"~/.prisma",
			},
		},

		// Railway
		{
			ID:       "railway",
			Name:     "Railway",
			Category: "cloud",
			Icon:     "🚂",
			ConfigPaths: []string{
				"~/.railway",
			},
		},

		// Vercel
		{
			ID:       "vercel",
			Name:     "Vercel CLI",
			Category: "cloud",
			Icon:     "▲",
			ConfigPaths: []string{
				"~/.vercel",
			},
		},

		// Netlify
		{
			ID:       "netlify",
			Name:     "Netlify CLI",
			Category: "cloud",
			Icon:     "🌐",
			ConfigPaths: []string{
				"~/.netlify",
			},
		},

		// Fly.io
		{
			ID:       "fly",
			Name:     "Fly.io CLI",
			Category: "cloud",
			Icon:     "✈️",
			ConfigPaths: []string{
				"~/.fly",
			},
		},

		// Render
		{
			ID:       "render",
			Name:     "Render CLI",
			Category: "cloud",
			Icon:     "🎨",
			ConfigPaths: []string{
				"~/.render",
			},
		},

		// SST
		{
			ID:       "sst",
			Name:     "SST",
			Category: "cloud",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.sst",
			},
		},

		// Wrangler (Cloudflare Workers)
		{
			ID:       "wrangler",
			Name:     "Wrangler",
			Category: "cloud",
			Icon:     "☁️",
			ConfigPaths: []string{
				"~/.wrangler",
			},
		},

		// Astro
		{
			ID:       "astro",
			Name:     "Astro",
			Category: "javascript",
			Icon:     "🚀",
			ConfigPaths: []string{
				"~/.config/astro",
			},
		},

		// SvelteKit
		{
			ID:       "sveltekit",
			Name:     "SvelteKit",
			Category: "javascript",
			Icon:     "🔥",
			ConfigPaths: []string{
				"~/.config/svelte",
			},
		},

		// Tauri
		{
			ID:       "tauri",
			Name:     "Tauri",
			Category: "dev",
			Icon:     "🦀",
			ConfigPaths: []string{
				"~/.tauri",
			},
		},

		// Wails
		{
			ID:       "wails",
			Name:     "Wails",
			Category: "dev",
			Icon:     "🐋",
			ConfigPaths: []string{
				"~/.wails",
			},
		},

		// Encore
		{
			ID:       "encore",
			Name:     "Encore",
			Category: "dev",
			Icon:     "🎭",
			ConfigPaths: []string{
				"~/.encore",
			},
		},

		// Modal
		{
			ID:       "modal",
			Name:     "Modal",
			Category: "ai",
			Icon:     "🖼️",
			ConfigPaths: []string{
				"~/.modal.toml",
			},
		},

		// Replicate
		{
			ID:       "replicate",
			Name:     "Replicate",
			Category: "ai",
			Icon:     "🔄",
			ConfigPaths: []string{
				"~/.replicate",
			},
		},

		// Together AI
		{
			ID:       "together",
			Name:     "Together AI",
			Category: "ai",
			Icon:     "🤝",
			ConfigPaths: []string{
				"~/.together",
			},
		},

		// Fireworks AI
		{
			ID:       "fireworks",
			Name:     "Fireworks AI",
			Category: "ai",
			Icon:     "🎆",
			ConfigPaths: []string{
				"~/.fireworks",
			},
		},

		// Anyscale
		{
			ID:       "anyscale",
			Name:     "Anyscale",
			Category: "ai",
			Icon:     "📈",
			ConfigPaths: []string{
				"~/.anyscale",
			},
		},

		// Weights & Biases
		{
			ID:       "wandb",
			Name:     "Weights & Biases",
			Category: "ai",
			Icon:     "📊",
			ConfigPaths: []string{
				"~/.config/wandb",
			},
		},

		// MLflow
		{
			ID:       "mlflow",
			Name:     "MLflow",
			Category: "ai",
			Icon:     "🔬",
			ConfigPaths: []string{
				"~/.mlflow",
			},
		},

		// DVC
		{
			ID:       "dvc",
			Name:     "DVC",
			Category: "ai",
			Icon:     "📦",
			ConfigPaths: []string{
				"~/.config/dvc",
			},
		},

		// LangChain
		{
			ID:       "langchain",
			Name:     "LangChain",
			Category: "ai",
			Icon:     "🔗",
			ConfigPaths: []string{
				"~/.langchain",
			},
		},

		// LlamaIndex
		{
			ID:       "llamaindex",
			Name:     "LlamaIndex",
			Category: "ai",
			Icon:     "🦙",
			ConfigPaths: []string{
				"~/.llamaindex",
			},
		},

		// OpenAI CLI
		{
			ID:       "openai",
			Name:     "OpenAI CLI",
			Category: "ai",
			Icon:     "🧠",
			ConfigPaths: []string{
				"~/.config/openai",
			},
		},

		// Anthropic CLI
		{
			ID:       "anthropic",
			Name:     "Anthropic CLI",
			Category: "ai",
			Icon:     "🤖",
			ConfigPaths: []string{
				"~/.config/anthropic",
			},
		},

		// Google AI CLI
		{
			ID:       "googleai",
			Name:     "Google AI CLI",
			Category: "ai",
			Icon:     "🔵",
			ConfigPaths: []string{
				"~/.config/google-ai",
			},
		},

		// Cohere
		{
			ID:       "cohere",
			Name:     "Cohere CLI",
			Category: "ai",
			Icon:     "🧵",
			ConfigPaths: []string{
				"~/.cohere",
			},
		},

		// Hugging Face
		{
			ID:       "huggingface",
			Name:     "Hugging Face",
			Category: "ai",
			Icon:     "🤗",
			ConfigPaths: []string{
				"~/.cache/huggingface",
				"~/.huggingface",
			},
		},
	}
}

// ScanAll returns all apps including not installed ones
func (s *Scanner) ScanAll() ([]*models.App, error) {
	defs, err := s.loadDefinitions()
	if err != nil {
		defs = s.getBuiltinDefinitions()
	}

	var apps []*models.App

	for _, def := range defs {
		app := models.NewApp(def)

		for _, configPath := range def.ConfigPaths {
			expandedPath := s.expandPath(configPath)

			if s.pathExists(expandedPath) {
				app.Installed = true

				files, err := s.collectFiles(expandedPath, def.EncryptedFiles)
				if err == nil {
					app.Files = append(app.Files, files...)
				}
			}
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// loadDefinitions loads app definitions from YAML
func (s *Scanner) loadDefinitions() ([]models.AppDefinition, error) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return nil, err
	}

	var config models.AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config.Apps, nil
}

// expandPath expands ~ to home directory
func (s *Scanner) expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(s.homeDir, path[2:])
	}
	return path
}

// pathExists checks if a path exists
func (s *Scanner) pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Maximum files to collect per directory (to avoid scanning huge directories)
const maxFilesPerDir = 200

// Maximum depth to scan in directories
const maxScanDepth = 5

// collectFiles collects all files from a path
func (s *Scanner) collectFiles(path string, encryptedFiles []string) ([]models.File, error) {
	var files []models.File

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Single file
		file, err := models.NewFile(path, filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		file.Encrypted = s.isEncrypted(file.Name, encryptedFiles)
		files = append(files, *file)
		return files, nil
	}

	// Directory - use parent as basePath so RelPath includes the folder name
	basePath := filepath.Dir(path)
	baseDepth := strings.Count(path, string(os.PathSeparator))
	folderName := filepath.Base(path)

	// Add the root directory as a file entry
	dirFile, err := models.NewFile(path, basePath)
	if err == nil {
		dirFile.IsDir = true
		dirFile.RelPath = folderName
		files = append(files, *dirFile)
	}

	fileCount := 0

	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip the root directory itself (already added)
		if p == path {
			return nil
		}

		// Check depth limit
		currentDepth := strings.Count(p, string(os.PathSeparator)) - baseDepth
		if d.IsDir() && currentDepth >= maxScanDepth {
			return filepath.SkipDir
		}

		// Skip hidden directories (except the root)
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != path {
			return filepath.SkipDir
		}

		// Skip common unwanted files/dirs
		if s.shouldSkip(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file limit
		if fileCount >= maxFilesPerDir {
			return filepath.SkipAll
		}

		// Add both files and directories - use parent of root as basePath
		// so RelPath includes the root folder name
		file, err := models.NewFile(p, basePath)
		if err == nil {
			file.IsDir = d.IsDir()
			file.Encrypted = s.isEncrypted(file.Name, encryptedFiles)
			files = append(files, *file)
			fileCount++
		}

		return nil
	})

	return files, err
}

// isEncrypted checks if a file should be encrypted
func (s *Scanner) isEncrypted(filename string, encryptedFiles []string) bool {
	for _, ef := range encryptedFiles {
		if filename == ef || strings.HasSuffix(filename, ef) {
			return true
		}
	}
	return false
}

// shouldSkip returns true if the file/dir should be skipped
func (s *Scanner) shouldSkip(name string) bool {
	for _, pattern := range skipPatterns {
		if name == pattern {
			return true
		}
	}
	// Check for patterns like *.log, *.bak
	suffixes := []string{".log", ".bak", ".backup", ".swp", ".swo"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// GroupByCategory groups apps by their category
func GroupByCategory(apps []*models.App) map[string][]*models.App {
	groups := make(map[string][]*models.App)

	for _, app := range apps {
		category := app.Category
		if category == "" {
			category = "other"
		}
		groups[category] = append(groups[category], app)
	}

	return groups
}

// CategoryOrder returns the preferred order of categories
func CategoryOrder() []string {
	return []string{
		"ai",
		"terminal",
		"shell",
		"editor",
		"git",
		"dev",
		"productivity",
		"cli",
		"discovered",
		"other",
	}
}

// CategoryNames returns display names for categories
func CategoryNames() map[string]string {
	return map[string]string{
		"ai":           "AI Tools",
		"terminal":     "Terminals",
		"shell":        "Shells",
		"editor":       "Editors",
		"git":          "Git",
		"dev":          "Dev Tools",
		"productivity": "Productivity",
		"cli":          "CLI Tools",
		"discovered":   "Discovered",
		"other":        "Other",
	}
}

// CategoryIcons returns icons for each category
func CategoryIcons() map[string]string {
	return map[string]string{
		"ai":           "🤖",
		"terminal":     "💻",
		"shell":        "🐚",
		"editor":       "📝",
		"git":          "🔀",
		"dev":          "🛠️",
		"productivity": "⚡",
		"cli":          "⌨️",
		"discovered":   "🔍",
		"other":        "📦",
	}
}

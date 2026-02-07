package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dotsync/internal/brew"
	"dotsync/internal/config"
	"dotsync/internal/git"
	"dotsync/internal/models"
	"dotsync/internal/scanner"
	"dotsync/internal/sync"
	"dotsync/internal/ui"
	"dotsync/internal/ui/components"

	// New modules for backup mode features
	"dotsync/internal/backup"
	"dotsync/internal/editor"
	"dotsync/internal/modes"
	"dotsync/internal/quicksync"
	"dotsync/internal/suggestions"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Version info (set by ldflags)
var (
	version   = "dev"
	buildTime = "unknown"
	debugMode = false // Enable with --debug flag
)

// debugLog logs a message if debug mode is enabled
func debugLog(format string, args ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// Screen represents different screens in the app
type Screen int

const (
	ScreenSetup Screen = iota
	ScreenMain
	ScreenScanning
	ScreenSyncing    // Sync progress screen
	ScreenConfirm    // Confirmation screen before pull
	ScreenHelp
	ScreenDiff       // Diff viewer screen
	ScreenGit        // Git operations screen
	ScreenMerge      // Merge conflict resolution screen
	ScreenCommit     // Commit message input screen
	ScreenPreview    // File preview screen
	ScreenSettings   // Settings screen
	ScreenRestore    // Restore from another machine
	ScreenQuickSync  // Quick sync progress/result
)

// Panel represents which panel is focused
type Panel int

const (
	PanelApps Panel = iota
	PanelFiles
)

// SetupStep represents steps in setup wizard
type SetupStep int

const (
	SetupWelcome SetupStep = iota
	SetupPath
	SetupConfirm
)

// SettingsField represents which field is being edited in settings
type SettingsField int

const (
	SettingsDotfilesPath SettingsField = iota
	SettingsBackupPath
	SettingsFieldCount // Used to wrap around
)

// SyncAction represents the type of sync action
type SyncAction int

const (
	ActionPush SyncAction = iota // Local -> Dotfiles
	ActionPull                   // Dotfiles -> Local
)

// ConfirmOption represents options in confirmation dialogs
type ConfirmOption int

const (
	ConfirmProceed ConfirmOption = iota // Proceed with the operation
	ConfirmBackup                       // Backup first, then proceed (pull only)
	ConfirmCancel                       // Cancel operation
)

// Model is the main application model
type Model struct {
	config       *config.Config
	apps         []*models.App
	stateManager *sync.StateManager

	// UI Components
	appList     *components.AppList
	fileList    *components.FileList
	diffView    *components.DiffView
	mergeView   *components.MergeView
	gitPanel    *components.GitPanel
	filePreview *components.FilePreview
	spinner     spinner.Model
	progress    progress.Model
	help        help.Model
	helpVP      viewport.Model
	keys        ui.KeyMap
	textInput   textinput.Model
	textArea    textarea.Model // For multi-line commit messages

	// State
	screen       Screen
	focusedPanel Panel
	status       string
	width        int
	height       int
	syncing      bool
	syncResults  []sync.ExportResult

	// Sync progress tracking
	syncTotal   int
	syncCurrent int
	syncAction  string

	// Setup wizard
	setupStep SetupStep

	// Settings screen
	settingsField   SettingsField
	settingsEditing bool // Whether we're editing a field

	// Confirmation dialog
	confirmAction SyncAction
	confirmCursor int
	fileDiffs     []FileDiff

	// Diff viewer state
	currentDiffFile *models.File
	currentDiffApp  *models.App

	// Search state
	searchMode   bool
	searchQuery  string
	filteredApps []*models.App

	// Category filter
	categoryFilter string

	// Undo state for selections
	lastAppSelections  map[string]bool // app ID -> selected state
	lastFileSelections map[string]bool // file path -> selected state
	canUndo            bool

	// New: Backup mode features
	modesConfig   *modes.ModesConfig
	quickSync     *quicksync.QuickSync
	backupManager *backup.BackupManager
	suggestion    *suggestions.Suggestion
	editorInst    editor.Editor

	// New: Quick sync state
	quickSyncResult *quicksync.Result

	// New: Restore dialog state
	restoreMachines     []backup.Machine
	restoreFiles        []backup.RestorableFile
	restoreCursor       int
	restoreSelectedMachine string

	err error
}

// FileDiff represents the diff between local and dotfiles version
type FileDiff struct {
	File           models.File
	LocalExists    bool
	DotfileExists  bool
	LocalModTime   string
	DotfileModTime string
	Status         string // "new", "modified", "same", "missing"
}

// Messages
type scanCompleteMsg struct {
	apps []*models.App
	err  error
}

type syncCompleteMsg struct {
	results []sync.ExportResult
	err     error
	action  string
}

type syncProgressMsg struct {
	current int
	total   int
	file    string
}

type configSavedMsg struct {
	err error
}

type diffCompleteMsg struct {
	diffs []FileDiff
	err   error
}

type refreshCompleteMsg struct {
	apps           []*models.App
	err            error
	categoryFilter string
}

func New() *Model {
	cfg, _ := config.Load()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ui.ProgressStyle

	// Initialize progress bar with gradient
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	ti := textinput.New()
	ti.Placeholder = "~/dotfiles"
	ti.CharLimit = 256
	ti.Width = 50

	// Initialize textarea for commit messages
	ta := textarea.New()
	ta.Placeholder = "Enter commit message..."
	ta.SetWidth(50)
	ta.SetHeight(4)
	ta.ShowLineNumbers = false

	// Initialize state manager for conflict detection
	stateManager := sync.NewStateManager(config.ConfigDir())
	_ = stateManager.Load() // Load existing state if available

	// Initialize modes config for sync/backup mode
	modesCfg, _ := modes.Load()

	// Initialize backup manager
	backupMgr := backup.New(cfg, modesCfg)

	// Initialize quick sync
	qs := quicksync.New(cfg, modesCfg)

	// Initialize editor (auto-detect)
	editorInst, _ := editor.Detect(nil)

	m := &Model{
		config:        cfg,
		stateManager:  stateManager,
		modesConfig:   modesCfg,
		backupManager: backupMgr,
		quickSync:     qs,
		editorInst:    editorInst,
		appList:       components.NewAppList(nil),
		fileList:      components.NewFileList(),
		diffView:      components.NewDiffView(),
		mergeView:     components.NewMergeView(),
		gitPanel:      components.NewGitPanel(),
		filePreview:   components.NewFilePreview(),
		spinner:       s,
		progress:      prog,
		help:          help.New(),
		keys:          ui.DefaultKeyMap(),
		textInput:     ti,
		textArea:      ta,
		screen:        ScreenMain,
		focusedPanel:  PanelApps,
		status:        "Ready",
		width:         80,
		height:        24,
		setupStep:     SetupWelcome,
	}

	if cfg.FirstRun {
		m.screen = ScreenSetup
	}

	return m
}

func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.spinner.Tick)

	if m.screen == ScreenMain {
		cmds = append(cmds, m.scanApps)
	}

	return tea.Batch(cmds...)
}

func (m *Model) scanApps() tea.Msg {
	startTime := time.Now()
	debugLog("Starting scan...")

	s := scanner.New(m.config.AppsConfig)

	debugLog("Scanner created, starting parallel scan...")
	scanStart := time.Now()
	apps, err := s.Scan()
	debugLog("Scan completed in %v, found %d apps", time.Since(scanStart), len(apps))

	if err != nil {
		debugLog("Scan error: %v", err)
		return scanCompleteMsg{apps: apps, err: err}
	}

	debugLog("Starting hash-based sync status update...")
	hashStart := time.Now()
	for i, app := range apps {
		debugLog("  [%d/%d] Updating sync status for %s (%d files)...", i+1, len(apps), app.Name, len(app.Files))
		sync.UpdateSyncStatusWithHashes(app, m.config.DotfilesPath, m.stateManager)
	}
	debugLog("Sync status update completed in %v", time.Since(hashStart))

	debugLog("Total scan time: %v", time.Since(startTime))
	return scanCompleteMsg{apps: apps, err: err}
}

func (m *Model) pushApps() tea.Msg {
	exporter := sync.NewExporter(m.config)
	results, err := exporter.ExportAll(m.apps)
	return syncCompleteMsg{results: results, err: err, action: "push"}
}

func (m *Model) pullApps() tea.Msg {
	importer := sync.NewImporter(m.config)
	var results []sync.ExportResult
	importResults, err := importer.ImportAll(m.apps)

	for _, r := range importResults {
		results = append(results, sync.ExportResult{
			App:     r.App,
			File:    r.File,
			Success: r.Success,
			Error:   r.Error,
		})
	}

	return syncCompleteMsg{results: results, err: err, action: "pull"}
}

func (m *Model) scanDiffs() tea.Msg {
	var diffs []FileDiff

	selected := m.appList.SelectedApps()
	for _, app := range selected {
		if !app.Selected {
			continue
		}

		appDir := filepath.Join(m.config.DotfilesPath, app.ID)

		for _, file := range app.Files {
			if !file.Selected {
				continue
			}

			diff := FileDiff{
				File: file,
			}

			// Check local file
			if info, err := os.Stat(file.Path); err == nil {
				diff.LocalExists = true
				diff.LocalModTime = info.ModTime().Format("2006-01-02 15:04")
			}

			// Check dotfiles version
			dotfilePath := filepath.Join(appDir, file.RelPath)
			if info, err := os.Stat(dotfilePath); err == nil {
				diff.DotfileExists = true
				diff.DotfileModTime = info.ModTime().Format("2006-01-02 15:04")
			}

			// Determine status
			if !diff.DotfileExists {
				diff.Status = "not in dotfiles"
			} else if !diff.LocalExists {
				diff.Status = "new (will create)"
			} else if diff.LocalModTime != diff.DotfileModTime {
				diff.Status = "different"
			} else {
				diff.Status = "same"
			}

			diffs = append(diffs, diff)
		}
	}

	return diffCompleteMsg{diffs: diffs}
}

func (m *Model) scanPushDiffs() tea.Msg {
	var diffs []FileDiff

	selected := m.appList.SelectedApps()
	for _, app := range selected {
		if !app.Selected {
			continue
		}

		appDir := filepath.Join(m.config.DotfilesPath, app.ID)

		for _, file := range app.Files {
			if !file.Selected {
				continue
			}

			diff := FileDiff{
				File: file,
			}

			// Check local file
			if info, err := os.Stat(file.Path); err == nil {
				diff.LocalExists = true
				diff.LocalModTime = info.ModTime().Format("2006-01-02 15:04")
			}

			// Check dotfiles version
			dotfilePath := filepath.Join(appDir, file.RelPath)
			if info, err := os.Stat(dotfilePath); err == nil {
				diff.DotfileExists = true
				diff.DotfileModTime = info.ModTime().Format("2006-01-02 15:04")
			}

			// Determine status for push
			if !diff.LocalExists {
				diff.Status = "missing locally"
			} else if !diff.DotfileExists {
				diff.Status = "new (will create)"
			} else if diff.LocalModTime != diff.DotfileModTime {
				diff.Status = "will overwrite"
			} else {
				diff.Status = "same"
			}

			diffs = append(diffs, diff)
		}
	}

	return diffCompleteMsg{diffs: diffs}
}

func (m *Model) saveConfig() tea.Msg {
	err := m.config.Save()
	if err == nil {
		err = m.config.EnsureDirectories()
	}
	return configSavedMsg{err: err}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updatePanelSizes()
		if m.screen == ScreenHelp {
			m.helpVP.Width = m.width - 4
			m.helpVP.Height = m.height - 4
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.MouseMsg:
		// Forward mouse events to file preview when in preview screen
		if m.screen == ScreenPreview {
			var cmd tea.Cmd
			m.filePreview, cmd = m.filePreview.Update(msg)
			return m, cmd
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmds = append(cmds, cmd)

	case scanCompleteMsg:
		m.screen = ScreenMain
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: %v", msg.err)
			m.err = msg.err
		} else {
			m.apps = msg.apps
			m.appList.SetApps(m.apps)
			m.status = fmt.Sprintf("Found %d apps with configs", len(m.apps))
		}

	case syncCompleteMsg:
		m.screen = ScreenMain
		m.syncing = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: %v", msg.err)
		} else {
			success := 0
			for _, r := range msg.results {
				if r.Success {
					success++
					// Update sync state for successfully synced files
					if m.stateManager != nil && r.App != nil {
						localHash := r.File.LocalHash
						dotfilesHash := r.File.DotfilesHash

						// After sync, both hashes should be the same
						if msg.action == "push" {
							// After push, dotfiles now has the local content
							dotfilesHash = localHash
						} else {
							// After pull, local now has the dotfiles content
							localHash = dotfilesHash
						}

						if localHash != "" || dotfilesHash != "" {
							m.stateManager.SetFileState(r.App.ID, r.File.RelPath, localHash, dotfilesHash)
						}
					}
				}
			}

			// Save state after sync
			if m.stateManager != nil {
				_ = m.stateManager.Save()
			}

			action := "Pushed"
			nextHint := " • Press 'g' to commit changes"
			if msg.action == "pull" {
				action = "Pulled"
				nextHint = " • Configs restored successfully"
			}
			m.status = fmt.Sprintf("✓ %s %d/%d files%s", action, success, len(msg.results), nextHint)
		}
		m.syncResults = msg.results

	case syncProgressMsg:
		m.syncCurrent = msg.current
		m.syncTotal = msg.total
		m.status = fmt.Sprintf("Syncing: %s", msg.file)
		return m, nil

	case diffCompleteMsg:
		m.fileDiffs = msg.diffs
		m.screen = ScreenConfirm
		m.confirmCursor = 0

	case refreshCompleteMsg:
		m.screen = ScreenMain
		if msg.err != nil {
			m.status = fmt.Sprintf("Refresh error: %v", msg.err)
			m.err = msg.err
		} else {
			m.apps = msg.apps
			// Restore category filter if it was active
			if msg.categoryFilter != "" {
				m.categoryFilter = msg.categoryFilter
				var filtered []*models.App
				for _, app := range m.apps {
					if strings.ToLower(app.Category) == msg.categoryFilter {
						filtered = append(filtered, app)
					}
				}
				m.filteredApps = filtered
				m.appList.SetApps(filtered)
				m.status = fmt.Sprintf("Refreshed: %d apps (%s filter active)", len(filtered), msg.categoryFilter)
			} else {
				m.appList.SetApps(m.apps)
				m.status = fmt.Sprintf("Refreshed: %d apps found", len(m.apps))
			}
			m.updateFileList()
		}

	case configSavedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error saving config: %v", msg.err)
		} else {
			m.screen = ScreenScanning
			m.status = "Scanning for apps..."
			return m, m.scanApps
		}

	case quickSyncCompleteMsg:
		m.syncing = false
		if msg.result == nil {
			m.status = "Quick backup failed"
			return m, nil
		}

		if msg.result.Error != nil {
			m.status = fmt.Sprintf("Quick backup error: %v", msg.result.Error)
			return m, nil
		}

		m.status = msg.result.Summary()

		// If there are pending sync files, show count
		if msg.result.HasSyncPending() {
			pendingInfo := ""
			if msg.result.SyncLocalMod > 0 {
				pendingInfo += fmt.Sprintf(" %d to push", msg.result.SyncLocalMod)
			}
			if msg.result.SyncRemoteMod > 0 {
				pendingInfo += fmt.Sprintf(" %d to pull", msg.result.SyncRemoteMod)
			}
			if msg.result.SyncConflicts > 0 {
				pendingInfo += fmt.Sprintf(" %d conflicts", msg.result.SyncConflicts)
			}
			m.status += " | Sync files:" + pendingInfo
		}

	case conflictCheckMsg:
		if msg.detection == nil {
			m.status = "Conflict check failed"
			return m, nil
		}

		if msg.detection.IsAllSynced() {
			m.status = "All files are synced"
		} else {
			parts := []string{}
			if msg.detection.LocalModified > 0 {
				parts = append(parts, fmt.Sprintf("%d modified (push)", msg.detection.LocalModified))
			}
			if msg.detection.RemoteUpdated > 0 {
				parts = append(parts, fmt.Sprintf("%d outdated (pull)", msg.detection.RemoteUpdated))
			}
			if msg.detection.Conflicts > 0 {
				parts = append(parts, fmt.Sprintf("%d conflicts", msg.detection.Conflicts))
			}
			m.status = "Status: " + strings.Join(parts, ", ")
		}

	case editorOpenedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Editor error: %v", msg.err)
		} else {
			m.status = "Editor closed"
		}
	}

	if m.screen == ScreenSetup && m.setupStep == SetupPath {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenSetup:
		return m.handleSetupKeys(msg)
	case ScreenConfirm:
		return m.handleConfirmKeys(msg)
	case ScreenDiff:
		return m.handleDiffKeys(msg)
	case ScreenMerge:
		return m.handleMergeKeys(msg)
	case ScreenGit:
		return m.handleGitKeys(msg)
	case ScreenCommit:
		return m.handleCommitKeys(msg)
	case ScreenPreview:
		return m.handlePreviewKeys(msg)
	case ScreenHelp:
		if key.Matches(msg, m.keys.Escape, m.keys.Help, m.keys.Quit) {
			m.screen = ScreenMain
			return m, nil
		}
		// Forward to viewport for scrolling
		var cmd tea.Cmd
		m.helpVP, cmd = m.helpVP.Update(msg)
		return m, cmd
	case ScreenSettings:
		return m.handleSettingsKeys(msg)
	case ScreenScanning:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	case ScreenSyncing:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}

	if m.syncing {
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}

	return m.handleMainKeys(msg)
}

func (m *Model) handleMainKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if m.searchMode {
		return m.handleSearchKeys(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		// Esc: clear active filters (search or category)
		if m.searchQuery != "" || m.categoryFilter != "" {
			return m.clearAllFilters()
		}
		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.screen = ScreenHelp
		m.helpVP = viewport.New(m.width-4, m.height-4)
		m.helpVP.SetContent(m.renderHelp())
		return m, nil

	case key.Matches(msg, m.keys.Tab, m.keys.ShiftTab):
		m.togglePanel()
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.handleNavigation(true)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.handleNavigation(false)
		return m, nil

	case key.Matches(msg, m.keys.PageUp):
		m.handlePageNavigation(true)
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		m.handlePageNavigation(false)
		return m, nil

	case key.Matches(msg, m.keys.Home):
		m.handleHomeEnd(true)
		return m, nil

	case key.Matches(msg, m.keys.End):
		m.handleHomeEnd(false)
		return m, nil

	case key.Matches(msg, m.keys.Space):
		m.handleToggle()
		return m, nil

	case key.Matches(msg, m.keys.SelectAll):
		m.handleSelectAll(true)
		return m, nil

	case key.Matches(msg, m.keys.DeselectAll):
		m.handleSelectAll(false)
		return m, nil

	case key.Matches(msg, m.keys.SelectMod):
		return m.handleSelectModified()

	case key.Matches(msg, m.keys.SelectOut):
		return m.handleSelectOutdated()

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Undo):
		return m.handleUndo()

	case key.Matches(msg, m.keys.Push):
		return m.handlePush()

	case key.Matches(msg, m.keys.Pull):
		return m.handlePull()

	case key.Matches(msg, m.keys.Scan):
		m.screen = ScreenScanning
		m.status = "Scanning..."
		return m, m.scanApps

	case key.Matches(msg, m.keys.Diff):
		return m.handleDiff()

	case key.Matches(msg, m.keys.Git):
		return m.handleGit()

	case key.Matches(msg, m.keys.Preview):
		return m.handlePreview()

	case key.Matches(msg, m.keys.Brewfile):
		return m.handleBrewfile()

	case msg.String() == ",": // Comma for Settings (like Vim/tmux convention)
		return m.handleSettings()

	case msg.String() == "l", msg.String() == "right":
		// Expand directory or enter files panel
		if m.focusedPanel == PanelFiles {
			m.fileList.ToggleExpand()
		} else {
			m.togglePanel()
			m.updateFileList()
		}
		return m, nil

	case msg.String() == "h", msg.String() == "left":
		// Collapse directory or go back to apps panel
		if m.focusedPanel == PanelFiles {
			// Try to collapse current directory, if not a directory go to apps panel
			node := m.fileList.CurrentNode()
			if node != nil && node.IsDir && node.Expanded {
				m.fileList.ToggleExpand()
			} else {
				m.togglePanel()
				m.updateFileList()
			}
		} else {
			// Already in apps panel, do nothing
		}
		return m, nil

	case msg.String() == "/":
		// Enter search mode
		// Enter search mode
		m.searchMode = true
		m.searchQuery = ""
		m.textInput.SetValue("")
		m.textInput.Placeholder = "Search apps..."
		m.textInput.Focus()
		m.status = "Type to search, Enter to confirm, Esc to cancel"
		return m, textinput.Blink

	case msg.String() == "1":
		return m.filterByCategory("ai")
	case msg.String() == "2":
		return m.filterByCategory("shell")
	case msg.String() == "3":
		return m.filterByCategory("editor")
	case msg.String() == "4":
		return m.filterByCategory("terminal")
	case msg.String() == "5":
		return m.filterByCategory("git")
	case msg.String() == "6":
		return m.filterByCategory("dev")
	case msg.String() == "7":
		return m.filterByCategory("cli")
	case msg.String() == "8":
		return m.filterByCategory("productivity")
	case msg.String() == "9":
		return m.filterByCategory("cloud")
	case msg.String() == "0":
		// Clear category filter
		return m.clearCategoryFilter()

	// New key bindings for backup mode features
	case key.Matches(msg, m.keys.QuickSync): // Q (Shift+Q): Quick Backup
		return m.handleQuickSync()

	case key.Matches(msg, m.keys.OpenEditor): // e: Open in Editor
		return m.handleOpenEditor()

	case key.Matches(msg, m.keys.CheckConflict): // c: Check conflicts
		return m.handleCheckConflicts()

	case key.Matches(msg, m.keys.ToggleMode): // t: Toggle mode
		return m.handleToggleMode()

	case key.Matches(msg, m.keys.Restore): // R (Shift+R): Open Restore dialog
		return m.handleRestore()

	case msg.String() == "P": // Shift+P: Push + Commit
		return m.handlePushAndCommit()
	}

	return m, nil
}

func (m *Model) handleNavigation(up bool) {
	if m.focusedPanel == PanelApps {
		if up {
			m.appList.MoveUp()
		} else {
			m.appList.MoveDown()
		}
		m.updateFileList()
	} else {
		if up {
			m.fileList.MoveUp()
		} else {
			m.fileList.MoveDown()
		}
	}
}

func (m *Model) handlePageNavigation(up bool) {
	if m.focusedPanel == PanelApps {
		if up {
			m.appList.PageUp()
		} else {
			m.appList.PageDown()
		}
		m.updateFileList()
	} else {
		if up {
			m.fileList.PageUp()
		} else {
			m.fileList.PageDown()
		}
	}
}

func (m *Model) handleHomeEnd(home bool) {
	if m.focusedPanel == PanelApps {
		if home {
			m.appList.GoToFirst()
		} else {
			m.appList.GoToLast()
		}
		m.updateFileList()
	} else {
		if home {
			m.fileList.GoToFirst()
		} else {
			m.fileList.GoToLast()
		}
	}
}

func (m *Model) handleToggle() {
	if m.focusedPanel == PanelApps {
		m.appList.Toggle()
	} else {
		m.fileList.Toggle()
		m.syncFilesToApp()
	}
}

func (m *Model) handleSelectAll(selectAll bool) {
	m.saveSelectionState() // Save before changing
	if m.focusedPanel == PanelApps {
		if selectAll {
			m.appList.SelectAll()
		} else {
			m.appList.DeselectAll()
		}
	} else {
		if selectAll {
			m.fileList.SelectAll()
		} else {
			m.fileList.DeselectAll()
		}
		m.syncFilesToApp()
	}
}

func (m *Model) syncFilesToApp() {
	if app := m.appList.Current(); app != nil {
		app.Files = m.fileList.Files
	}
}

func (m *Model) handlePush() (tea.Model, tea.Cmd) {
	selectedApps := m.appList.SelectedApps()
	if len(selectedApps) == 0 {
		m.status = "No apps selected"
		return m, nil
	}

	// Count selected files
	fileCount := 0
	for _, app := range selectedApps {
		for _, file := range app.Files {
			if file.Selected {
				fileCount++
			}
		}
	}

	if fileCount == 0 {
		m.status = "No files selected"
		return m, nil
	}

	// Show confirmation dialog
	m.confirmAction = ActionPush
	m.status = "Scanning files to push..."
	return m, m.scanPushDiffs
}

func (m *Model) handlePull() (tea.Model, tea.Cmd) {
	if len(m.appList.SelectedApps()) == 0 {
		m.status = "No apps selected"
		return m, nil
	}
	if !m.config.DotfilesExists() {
		m.status = "No dotfiles found. Push first!"
		return m, nil
	}
	m.confirmAction = ActionPull
	m.status = "Scanning differences..."
	return m, m.scanDiffs
}

func (m *Model) handleDiff() (tea.Model, tea.Cmd) {
	// Get current selected file
	if m.focusedPanel != PanelFiles {
		m.status = "Select a file first (Tab to switch panel)"
		return m, nil
	}

	currentFile := m.fileList.Current()
	if currentFile == nil {
		m.status = "No file selected"
		return m, nil
	}

	currentApp := m.appList.Current()
	if currentApp == nil {
		m.status = "No app selected"
		return m, nil
	}

	m.currentDiffFile = currentFile
	m.currentDiffApp = currentApp

	// Compute diff
	localPath := currentFile.Path
	dotfilePath := filepath.Join(m.config.DotfilesPath, currentApp.ID, currentFile.RelPath)

	diffResult, err := sync.ComputeDiff(localPath, dotfilePath)
	if err != nil {
		m.status = fmt.Sprintf("Diff error: %v", err)
		return m, nil
	}

	m.diffView.SetDiff(diffResult, localPath, dotfilePath)
	m.diffView.Width = m.width - 4
	m.diffView.Height = m.height - 6
	m.screen = ScreenDiff
	m.status = "Viewing diff"

	return m, nil
}

func (m *Model) handleGit() (tea.Model, tea.Cmd) {
	// Auto-create directory and init git if needed
	if !m.config.DotfilesExists() {
		if err := os.MkdirAll(m.config.DotfilesPath, 0755); err != nil {
			m.status = fmt.Sprintf("Cannot create dotfiles dir: %v", err)
			return m, nil
		}
	}
	if !m.config.IsGitRepo() {
		if err := m.config.InitGitRepo(); err != nil {
			m.status = fmt.Sprintf("Cannot init git: %v", err)
			return m, nil
		}
		m.status = "Git repository initialized"
	}

	// Initialize git panel with repository
	repo := git.NewRepo(m.config.DotfilesPath)
	m.gitPanel.SetRepo(repo)
	m.gitPanel.Width = m.width - 4
	m.gitPanel.Height = m.height - 6
	m.screen = ScreenGit
	if m.status != "Git repository initialized" {
		m.status = "Git operations"
	}

	return m, nil
}

func (m *Model) handleBrewfile() (tea.Model, tea.Cmd) {
	// Export Brewfile to dotfiles directory
	brewDir := filepath.Join(m.config.DotfilesPath, "homebrew")

	path, err := brew.ExportBrewfile(brewDir)
	if err != nil {
		m.status = fmt.Sprintf("Brewfile error: %v", err)
		return m, nil
	}

	// Get stats for status message
	info, _ := brew.GetInstalledPackages()
	formulae, casks, taps := info.Stats()

	m.status = fmt.Sprintf("Brewfile saved: %d formulae, %d casks, %d taps → %s",
		formulae, casks, taps, path)

	return m, nil
}

func (m *Model) handleSettings() (tea.Model, tea.Cmd) {
	m.screen = ScreenSettings
	m.settingsField = SettingsDotfilesPath
	m.settingsEditing = false
	m.status = "Settings - press Enter to edit, Esc to go back"
	return m, nil
}

func (m *Model) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsEditing {
		// We're editing a field
		switch msg.String() {
		case "enter":
			// Save the edited value
			value := m.textInput.Value()
			if value != "" {
				// Expand ~ to home directory
				if strings.HasPrefix(value, "~/") {
					homeDir, _ := os.UserHomeDir()
					value = filepath.Join(homeDir, value[2:])
				}

				switch m.settingsField {
				case SettingsDotfilesPath:
					m.config.DotfilesPath = value
				case SettingsBackupPath:
					m.config.BackupPath = value
				}

				// Save config
				if err := m.config.Save(); err != nil {
					m.status = fmt.Sprintf("Error saving config: %v", err)
				} else {
					// Ensure directories exist and init git if needed
					if err := m.config.EnsureDirectories(); err != nil {
						m.status = fmt.Sprintf("Saved, but dir error: %v", err)
					} else if m.settingsField == SettingsDotfilesPath {
						m.status = fmt.Sprintf("Dotfiles path set to %s", value)
					} else {
						m.status = "Settings saved!"
					}
				}
			}
			m.settingsEditing = false
			m.textInput.Blur()
			return m, nil

		case "esc":
			m.settingsEditing = false
			m.textInput.Blur()
			return m, nil

		default:
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
	}

	// Not editing - navigation mode
	switch msg.String() {
	case "q", "esc":
		m.screen = ScreenMain
		m.status = "Ready"
		return m, nil

	case "j", "down":
		m.settingsField = SettingsField((int(m.settingsField) + 1) % int(SettingsFieldCount))
		return m, nil

	case "k", "up":
		m.settingsField = SettingsField((int(m.settingsField) - 1 + int(SettingsFieldCount)) % int(SettingsFieldCount))
		return m, nil

	case "enter", " ":
		// Start editing the current field
		m.settingsEditing = true
		switch m.settingsField {
		case SettingsDotfilesPath:
			m.textInput.SetValue(m.config.DotfilesPath)
			m.textInput.Placeholder = "Enter dotfiles path..."
		case SettingsBackupPath:
			m.textInput.SetValue(m.config.BackupPath)
			m.textInput.Placeholder = "Enter backup path..."
		}
		m.textInput.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

func (m *Model) handlePreview() (tea.Model, tea.Cmd) {
	// Only preview when in Files panel
	if m.focusedPanel != PanelFiles {
		m.status = "Switch to Files panel to preview (Tab)"
		return m, nil
	}

	file := m.fileList.Current()
	if file == nil {
		m.status = "No file selected"
		return m, nil
	}

	// Set size and load file for preview
	m.filePreview.SetSize(m.width-4, m.height-4)
	if err := m.filePreview.Load(file.Path); err != nil {
		m.status = fmt.Sprintf("Cannot preview: %v", err)
		return m, nil
	}

	m.screen = ScreenPreview
	m.status = "File preview - j/k scroll, mouse wheel, q to close"
	return m, nil
}

func (m *Model) handlePreviewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape, m.keys.Quit):
		m.screen = ScreenMain
		m.status = "Ready"
		return m, nil

	default:
		// Forward all other keys to viewport for scrolling
		var cmd tea.Cmd
		m.filePreview, cmd = m.filePreview.Update(msg)
		return m, cmd
	}
}

func (m *Model) handleDiffKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape, m.keys.Quit):
		m.screen = ScreenMain
		m.status = "Ready"
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.diffView.ScrollUp()
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.diffView.ScrollDown()
		return m, nil

	case key.Matches(msg, m.keys.NextHunk):
		m.diffView.NextHunk()
		return m, nil

	case key.Matches(msg, m.keys.PrevHunk):
		m.diffView.PrevHunk()
		return m, nil

	case key.Matches(msg, m.keys.KeepLocal):
		// Keep local version - push to dotfiles
		if m.currentDiffFile != nil && m.currentDiffApp != nil {
			m.currentDiffFile.Selected = true
			m.screen = ScreenMain
			m.status = "Use 'p' to push local version to dotfiles"
		}
		return m, nil

	case key.Matches(msg, m.keys.UseDotfiles):
		// Use dotfiles version - pull to local
		if m.currentDiffFile != nil && m.currentDiffApp != nil {
			m.currentDiffFile.Selected = true
			m.screen = ScreenMain
			m.status = "Use 'l' to pull dotfiles version to local"
		}
		return m, nil

	case key.Matches(msg, m.keys.Merge):
		// Open merge tool
		return m.handleMerge()

	case msg.String() == "h":
		// Toggle syntax highlighting
		m.diffView.ToggleHighlight()
		return m, nil
	}

	return m, nil
}

func (m *Model) handleMerge() (tea.Model, tea.Cmd) {
	// Get current diff and create merge result
	if m.diffView.DiffResult == nil {
		m.status = "No diff to merge"
		return m, nil
	}

	if m.diffView.DiffResult.Identical {
		m.status = "Files are identical, no merge needed"
		return m, nil
	}

	// Create merge result from diff
	mergeResult := sync.NewMergeResult(
		m.diffView.DiffResult,
		m.diffView.LocalPath,
		m.diffView.DotfilePath,
	)

	m.mergeView.SetMerge(mergeResult)
	m.mergeView.Width = m.width - 4
	m.mergeView.Height = m.height - 6
	m.screen = ScreenMerge
	m.status = "Merge mode - resolve conflicts"

	return m, nil
}

func (m *Model) handleMergeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		// Go back to diff view
		m.screen = ScreenDiff
		m.status = "Back to diff view"
		return m, nil

	case key.Matches(msg, m.keys.Quit):
		m.screen = ScreenMain
		m.status = "Ready"
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.mergeView.ScrollUp()
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.mergeView.ScrollDown()
		return m, nil

	case key.Matches(msg, m.keys.NextHunk):
		m.mergeView.NextHunk()
		return m, nil

	case key.Matches(msg, m.keys.PrevHunk):
		m.mergeView.PrevHunk()
		return m, nil

	case key.Matches(msg, m.keys.KeepLocal):
		m.mergeView.ResolveCurrentKeepLocal()
		m.status = fmt.Sprintf("Resolved: keep local (%d/%d)",
			m.mergeView.MergeResult.ResolvedHunks,
			m.mergeView.MergeResult.TotalHunks)
		return m, nil

	case key.Matches(msg, m.keys.UseDotfiles):
		m.mergeView.ResolveCurrentUseDotfiles()
		m.status = fmt.Sprintf("Resolved: use dotfiles (%d/%d)",
			m.mergeView.MergeResult.ResolvedHunks,
			m.mergeView.MergeResult.TotalHunks)
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		// Save merged file if fully resolved
		if m.mergeView.IsFullyResolved() {
			if err := m.mergeView.MergeResult.WriteMergedFile(); err != nil {
				m.status = fmt.Sprintf("Error saving merge: %v", err)
				return m, nil
			}
			m.screen = ScreenMain
			m.status = "Merge saved successfully!"

			// Update sync state
			if m.stateManager != nil && m.currentDiffApp != nil && m.currentDiffFile != nil {
				// Recompute hash after merge
				newHash, _ := sync.ComputeFileHash(m.currentDiffFile.Path)
				m.stateManager.SetFileState(
					m.currentDiffApp.ID,
					m.currentDiffFile.RelPath,
					newHash,
					newHash,
				)
				_ = m.stateManager.Save()
			}
		} else {
			m.status = fmt.Sprintf("Resolve all hunks first (%d/%d)",
				m.mergeView.MergeResult.ResolvedHunks,
				m.mergeView.MergeResult.TotalHunks)
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) handleConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Determine max options based on action type
	maxOptions := 1 // Push has 2 options (0 and 1)
	if m.confirmAction == ActionPull {
		maxOptions = 2 // Pull has 3 options (0, 1, and 2)
	}

	switch msg.String() {
	case "up", "k":
		if m.confirmCursor > 0 {
			m.confirmCursor--
		}
	case "down", "j":
		if m.confirmCursor < maxOptions {
			m.confirmCursor++
		}
	case "enter", " ":
		if m.confirmAction == ActionPush {
			// Push confirmation
			switch ConfirmOption(m.confirmCursor) {
			case ConfirmProceed:
				m.syncing = true
				m.syncAction = "push"
				m.syncTotal = len(m.fileDiffs)
				m.syncCurrent = 0
				m.screen = ScreenSyncing
				m.status = fmt.Sprintf("Pushing %d files...", len(m.fileDiffs))
				return m, m.pushApps
			case ConfirmBackup: // Used as Cancel for push (index 1)
				m.screen = ScreenMain
				m.status = "Push cancelled"
			}
		} else {
			// Pull confirmation
			switch ConfirmOption(m.confirmCursor) {
			case ConfirmProceed:
				m.syncing = true
				m.syncAction = "pull"
				m.syncTotal = len(m.fileDiffs)
				m.syncCurrent = 0
				m.screen = ScreenSyncing
				m.status = "Backing up and pulling..."
				return m, m.pullApps
			case ConfirmBackup:
				m.syncing = true
				m.syncAction = "pull"
				m.syncTotal = len(m.fileDiffs)
				m.syncCurrent = 0
				m.screen = ScreenSyncing
				m.status = "Pulling (no backup)..."
				return m, m.pullApps
			case ConfirmCancel:
				m.screen = ScreenMain
				m.status = "Pull cancelled"
			}
		}
	case "esc", "q":
		m.screen = ScreenMain
		m.status = "Cancelled"
	case "1":
		m.confirmCursor = 0
	case "2":
		if maxOptions >= 1 {
			m.confirmCursor = 1
		}
	case "3":
		if maxOptions >= 2 {
			m.confirmCursor = 2
		}
	}
	return m, nil
}

func (m *Model) handleSetupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.setupStep {
	case SetupWelcome:
		switch msg.String() {
		case "enter", " ":
			m.setupStep = SetupPath
			m.textInput.SetValue(m.config.DotfilesPath)
			m.textInput.Focus()
			return m, textinput.Blink
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case SetupPath:
		switch msg.String() {
		case "enter":
			path := m.textInput.Value()
			if path == "" {
				path = m.config.DotfilesPath
			}
			if strings.HasPrefix(path, "~/") {
				homeDir, _ := os.UserHomeDir()
				path = filepath.Join(homeDir, path[2:])
			}
			m.config.DotfilesPath = path
			m.setupStep = SetupConfirm
			m.textInput.Blur()
		case "esc":
			m.setupStep = SetupWelcome
			m.textInput.Blur()
		case "1", "2", "3":
			paths := config.SuggestedPaths()
			idx := int(msg.String()[0] - '1')
			if idx < len(paths) {
				m.textInput.SetValue(paths[idx])
			}
		default:
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

	case SetupConfirm:
		switch msg.String() {
		case "enter", "y":
			m.config.FirstRun = false
			return m, m.saveConfig
		case "n", "esc":
			m.setupStep = SetupPath
			m.textInput.Focus()
			return m, textinput.Blink
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *Model) togglePanel() {
	if m.focusedPanel == PanelApps {
		m.focusedPanel = PanelFiles
		m.appList.Focused = false
		m.fileList.Focused = true
	} else {
		m.focusedPanel = PanelApps
		m.appList.Focused = true
		m.fileList.Focused = false
	}
}

func (m *Model) updateFileList() {
	if app := m.appList.Current(); app != nil {
		m.fileList.SetFiles(app.Files, app.Name)
	} else {
		m.fileList.Clear()
	}
}

func (m *Model) updatePanelSizes() {
	panelWidth := (m.width - 4) / 2
	panelHeight := m.height - 8

	m.appList.Width = panelWidth
	m.appList.Height = panelHeight
	m.fileList.Width = panelWidth
	m.fileList.Height = panelHeight
}

func (m *Model) View() string {
	switch m.screen {
	case ScreenSetup:
		return m.renderSetup()
	case ScreenConfirm:
		return m.renderConfirm()
	case ScreenDiff:
		return m.renderDiff()
	case ScreenMerge:
		return m.renderMerge()
	case ScreenGit:
		return m.renderGit()
	case ScreenCommit:
		return m.renderCommitDialog()
	case ScreenPreview:
		return m.renderPreview()
	case ScreenSettings:
		return m.renderSettings()
	default:
		return m.renderMain()
	}
}

func (m *Model) renderSetup() string {
	width := 60
	style := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Primary)

	var content string

	switch m.setupStep {
	case SetupWelcome:
		content = m.renderSetupWelcome()
	case SetupPath:
		content = m.renderSetupPath()
	case SetupConfirm:
		content = m.renderSetupConfirm()
	}

	box := style.Render(content)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func (m *Model) renderSetupWelcome() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Primary).
		Render("🔄 Welcome to Dotsync!")

	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("Dotsync helps you sync your dotfiles between machines.\n\n")
	b.WriteString("Features:\n")
	b.WriteString("  • Auto-detect installed apps and their configs\n")
	b.WriteString("  • Selective sync - choose which files to sync\n")
	b.WriteString("  • Support for 960+ apps out of the box\n")
	b.WriteString("  • Built-in git operations and branch switching\n")
	b.WriteString("  • Discovers unknown apps in ~/.config\n")
	b.WriteString("\n\n")
	b.WriteString(ui.HelpBarStyle.Render("Press ENTER to continue • q to quit"))

	return b.String()
}

func (m *Model) renderSetupPath() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Primary).
		Render("📁 Choose Dotfiles Location")

	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("Where do you want to store your dotfiles?\n\n")

	paths := config.SuggestedPaths()
	for i, path := range paths {
		prefix := fmt.Sprintf("[%d] ", i+1)
		exists := ""
		if _, err := os.Stat(path); err == nil {
			exists = " (exists)"
		}
		b.WriteString(ui.MutedStyle.Render(prefix))
		b.WriteString(path)
		b.WriteString(ui.MutedStyle.Render(exists))
		b.WriteString("\n")
	}

	b.WriteString("\nOr enter custom path:\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(ui.HelpBarStyle.Render("1-3 quick select • ENTER confirm • ESC back"))

	return b.String()
}

func (m *Model) renderSetupConfirm() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Primary).
		Render("✓ Confirm Setup")

	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("Dotfiles will be stored at:\n")
	b.WriteString(ui.SelectedItemStyle.Render("  " + m.config.DotfilesPath))
	b.WriteString("\n\n")

	if _, err := os.Stat(m.config.DotfilesPath); err == nil {
		b.WriteString(ui.SyncedStyle.Render("✓ Directory exists\n"))
	} else {
		b.WriteString(ui.MutedStyle.Render("  Directory will be created\n"))
	}

	b.WriteString("\n")
	b.WriteString(ui.HelpBarStyle.Render("y/ENTER confirm • n/ESC go back • q quit"))

	return b.String()
}

func (m *Model) renderConfirm() string {
	width := 70

	// Different styling for push vs pull
	borderColor := ui.Warning
	var titleText string
	var descText string
	var filesLabel string

	if m.confirmAction == ActionPush {
		borderColor = ui.Primary
		titleText = "📤 Push to Dotfiles"
		descText = "This will copy your local configs to your dotfiles repository."
		filesLabel = "Files to push:"
	} else {
		titleText = "⚠️  Pull from Dotfiles"
		descText = "This will replace your local configs with versions from dotfiles."
		filesLabel = "Files to pull:"
	}

	style := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(borderColor).
		Render(titleText)

	b.WriteString(title)
	b.WriteString("\n\n")

	b.WriteString(descText)
	b.WriteString("\n\n")

	// Show files that will be affected
	b.WriteString(ui.PanelTitleStyle.Render(filesLabel))
	b.WriteString("\n")

	maxShow := 8
	for i, diff := range m.fileDiffs {
		if i >= maxShow {
			remaining := len(m.fileDiffs) - maxShow
			b.WriteString(ui.MutedStyle.Render(fmt.Sprintf("  ... and %d more files\n", remaining)))
			break
		}

		icon := "📄"
		if diff.File.IsDir {
			icon = "📁"
		}

		statusStyle := ui.MutedStyle
		switch diff.Status {
		case "new (will create)":
			statusStyle = ui.NewStyle
		case "different", "will overwrite":
			statusStyle = ui.ModifiedStyle
		case "not in dotfiles", "missing locally":
			statusStyle = ui.MissingStyle
		case "same":
			statusStyle = ui.SyncedStyle
		}

		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			icon,
			diff.File.Name,
			statusStyle.Render("("+diff.Status+")"),
		))
	}

	b.WriteString("\n")
	b.WriteString(ui.PanelTitleStyle.Render("Choose action:"))
	b.WriteString("\n")

	// Different options for push vs pull
	var options []struct {
		key   string
		label string
		desc  string
	}

	if m.confirmAction == ActionPush {
		options = []struct {
			key   string
			label string
			desc  string
		}{
			{"1", "Push", "Copy local configs to dotfiles repository"},
			{"2", "Cancel", "Go back without changes"},
		}
	} else {
		options = []struct {
			key   string
			label string
			desc  string
		}{
			{"1", "Backup & Pull", "Save current configs to backup folder, then pull"},
			{"2", "Pull Only", "Replace without backup (not recommended)"},
			{"3", "Cancel", "Go back without changes"},
		}
	}

	for i, opt := range options {
		cursor := "  "
		optStyle := ui.ItemStyle
		if i == m.confirmCursor {
			cursor = ui.CursorStyle.Render("> ")
			optStyle = ui.SelectedItemStyle
		}

		b.WriteString(cursor)
		b.WriteString(optStyle.Render(fmt.Sprintf("[%s] %s", opt.key, opt.label)))
		b.WriteString("\n")
		b.WriteString("      ")
		b.WriteString(ui.MutedStyle.Render(opt.desc))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(ui.HelpBarStyle.Render("↑↓ navigate • ENTER select • ESC cancel"))

	box := style.Render(b.String())

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func (m *Model) renderMain() string {
	var b strings.Builder

	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	switch m.screen {
	case ScreenScanning:
		// Nice loading screen with tips
		var lines []string

		// Title with spinner
		lines = append(lines, m.spinner.View()+" Scanning for apps...")
		lines = append(lines, "")

		// Scanning locations
		lines = append(lines, "Looking for configurations in:")
		lines = append(lines, "  • ~/.config/")
		lines = append(lines, "  • ~/Library/Application Support/")
		lines = append(lines, "  • Home directory dotfiles")
		lines = append(lines, "")

		// Show helpful tips with rotating animation
		tips := []string{
			"💡 Use / to search apps by name",
			"💡 Press 1-9 to filter by category",
			"💡 Press M to select modified, O for outdated",
			"💡 Press d to view file differences",
			"💡 Press g to access git operations",
			"💡 Press s to rescan at any time",
		}
		tipIndex := int(time.Now().Unix()/3) % len(tips)
		lines = append(lines, tips[tipIndex])

		// Join all lines
		scanContent := strings.Join(lines, "\n")

		// Create a styled box for scan content
		scanBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.Primary).
			Padding(1, 3).
			Render(scanContent)

		// Get box dimensions
		boxHeight := lipgloss.Height(scanBox)
		boxWidth := lipgloss.Width(scanBox)

		// Calculate padding to center
		availableHeight := m.height - 6 // header + status + help + newlines
		availableWidth := m.width - 2   // AppStyle padding

		topPad := (availableHeight - boxHeight) / 2
		if topPad < 0 {
			topPad = 0
		}
		leftPad := (availableWidth - boxWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		// Build centered content with explicit padding
		var scanOutput strings.Builder
		for i := 0; i < topPad; i++ {
			scanOutput.WriteString("\n")
		}
		// Add left padding to each line of the box
		for _, line := range strings.Split(scanBox, "\n") {
			scanOutput.WriteString(strings.Repeat(" ", leftPad))
			scanOutput.WriteString(line)
			scanOutput.WriteString("\n")
		}

		b.WriteString(scanOutput.String())

	case ScreenSyncing:
		// Sync progress screen with progress bar
		var syncContent strings.Builder
		action := "Pushing"
		if m.syncAction == "pull" {
			action = "Pulling"
		}
		syncContent.WriteString(fmt.Sprintf("%s %s files...\n\n", m.spinner.View(), action))

		// Progress bar
		var progressPercent float64
		if m.syncTotal > 0 {
			progressPercent = float64(m.syncCurrent) / float64(m.syncTotal)
		}
		syncContent.WriteString(m.progress.ViewAs(progressPercent) + "\n\n")
		syncContent.WriteString(ui.MutedStyle.Render(fmt.Sprintf("  %d / %d files", m.syncCurrent, m.syncTotal)))
		syncContent.WriteString("\n\n")
		syncContent.WriteString(ui.MutedStyle.Render(m.status))

		content := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height-6).
			Align(lipgloss.Center, lipgloss.Center).
			Render(syncContent.String())
		b.WriteString(content)

	case ScreenHelp:
		b.WriteString(m.helpVP.View())

	default:
		panels := lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.appList.View(),
			"  ",
			m.fileList.View(),
		)
		b.WriteString(panels)
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return ui.AppStyle.Render(b.String())
}

func (m *Model) renderHeader() string {
	title := ui.TitleStyle.Render("🔄 Dotsync")
	ver := ui.VersionStyle.Render("v" + version)
	path := ui.MutedStyle.Render("  " + m.config.DotfilesPath)

	// Show git branch if in a git repo
	gitInfo := ""
	if m.config.IsGitRepo() {
		repo := git.NewRepo(m.config.DotfilesPath)
		if branch := repo.CurrentBranch(); branch != "" {
			gitInfo = ui.MutedStyle.Render(" [" + branch + "]")
		}
	}

	return ui.HeaderStyle.Render(title + "  " + ver + path + gitInfo)
}

func (m *Model) renderStatusBar() string {
	selectedApps := m.appList.SelectedApps()
	totalApps := len(m.apps)

	// Count selected files across all selected apps
	selectedFiles := 0
	modifiedFiles := 0
	conflictFiles := 0
	for _, app := range selectedApps {
		for _, file := range app.Files {
			if file.Selected {
				selectedFiles++
			}
			// Count modified and conflict files
			switch file.ConflictType {
			case models.ConflictLocalModified, models.ConflictLocalNew:
				modifiedFiles++
			case models.ConflictBothModified:
				conflictFiles++
			}
		}
	}

	// Build stats string
	var stats []string
	stats = append(stats, fmt.Sprintf("Apps: %d/%d", len(selectedApps), totalApps))
	if selectedFiles > 0 {
		stats = append(stats, fmt.Sprintf("Files: %d", selectedFiles))
	}
	if modifiedFiles > 0 {
		stats = append(stats, fmt.Sprintf("Modified: %d", modifiedFiles))
	}
	if conflictFiles > 0 {
		stats = append(stats, ui.ConflictStyle.Render(fmt.Sprintf("⚡Conflicts: %d", conflictFiles)))
	}

	// Show current panel indicator
	panelIndicator := "📁"
	if m.focusedPanel == PanelFiles {
		panelIndicator = "📄"
	}

	// Style status message based on content
	styledStatus := ui.StatusTextStyle.Render(m.status)
	if strings.HasPrefix(m.status, "✓") {
		styledStatus = ui.RenderNotification("success", strings.TrimPrefix(m.status, "✓ "))
	} else if strings.HasPrefix(m.status, "Error") {
		styledStatus = ui.RenderNotification("error", m.status)
	} else if strings.Contains(m.status, "cancelled") || strings.Contains(m.status, "failed") {
		styledStatus = ui.RenderNotification("warning", m.status)
	}

	return ui.StatusBarStyle.Render(
		panelIndicator + " " + styledStatus + "  •  " + strings.Join(stats, "  •  "),
	)
}

func (m *Model) renderHelpBar() string {
	// Show different help bar based on current screen
	switch m.screen {
	case ScreenScanning:
		items := []string{
			ui.RenderHelpItem("q", "quit"),
		}
		return ui.HelpBarStyle.Render("⏳ Scanning... " + strings.Join(items, "  "))

	case ScreenSyncing:
		items := []string{
			ui.RenderHelpItem("q", "quit"),
		}
		return ui.HelpBarStyle.Render("🔄 Syncing... " + strings.Join(items, "  "))

	case ScreenHelp:
		scrollPct := fmt.Sprintf("%d%%", int(m.helpVP.ScrollPercent()*100))
		items := []string{
			ui.RenderHelpItem("↑↓/j/k", "scroll"),
			ui.RenderHelpItem("PgUp/PgDn", "page"),
			ui.RenderHelpItem("esc/?", "close"),
			ui.RenderHelpItem(scrollPct, ""),
		}
		return ui.HelpBarStyle.Render(strings.Join(items, "  "))
	}

	// Show different help bar when in search mode
	if m.searchMode {
		items := []string{
			ui.RenderHelpItem("↑↓", "navigate"),
			ui.RenderHelpItem("enter", "confirm"),
			ui.RenderHelpItem("esc", "cancel"),
		}
		return ui.HelpBarStyle.Render("🔍 " + m.textInput.View() + "  " + strings.Join(items, "  "))
	}

	// Show filter hint if category filter is active
	if m.categoryFilter != "" {
		items := []string{
			ui.RenderHelpItem("esc", "clear"),
			ui.RenderHelpItem("space", "select"),
			ui.RenderHelpItem("Q", "backup"),
			ui.RenderHelpItem("p", "push"),
			ui.RenderHelpItem("l", "pull"),
			ui.RenderHelpItem("?", "help"),
		}
		return ui.HelpBarStyle.Render("📁 " + m.categoryFilter + "  " + strings.Join(items, "  "))
	}

	// Show search filter hint if search is active
	if m.searchQuery != "" {
		items := []string{
			ui.RenderHelpItem("esc", "clear"),
			ui.RenderHelpItem("space", "select"),
			ui.RenderHelpItem("Q", "backup"),
			ui.RenderHelpItem("p", "push"),
			ui.RenderHelpItem("l", "pull"),
			ui.RenderHelpItem("?", "help"),
		}
		return ui.HelpBarStyle.Render("🔍 \"" + m.searchQuery + "\"  " + strings.Join(items, "  "))
	}

	// Context-sensitive help based on panel and selection
	var items []string

	// Check if we have selected items
	selectedApps := m.appList.SelectedApps()
	hasSelection := len(selectedApps) > 0

	if m.focusedPanel == PanelApps {
		if hasSelection {
			// Show sync actions when items are selected
			items = []string{
				ui.RenderHelpItem("Q", "backup"),
				ui.RenderHelpItem("p", "push"),
				ui.RenderHelpItem("l", "pull"),
				ui.RenderHelpItem("t", "mode"),
				ui.RenderHelpItem("tab", "→files"),
				ui.RenderHelpItem("?", "help"),
			}
		} else {
			// Show selection actions when nothing selected
			items = []string{
				ui.RenderHelpItem("space", "select"),
				ui.RenderHelpItem("a", "all"),
				ui.RenderHelpItem("M", "mod"),
				ui.RenderHelpItem("O", "outdated"),
				ui.RenderHelpItem("/", "search"),
				ui.RenderHelpItem("1-9", "filter"),
				ui.RenderHelpItem("?", "help"),
			}
		}
	} else {
		// Files panel - show file-specific actions
		if hasSelection {
			items = []string{
				ui.RenderHelpItem("Q", "backup"),
				ui.RenderHelpItem("p", "push"),
				ui.RenderHelpItem("l", "pull"),
				ui.RenderHelpItem("d", "diff"),
				ui.RenderHelpItem("e", "edit"),
				ui.RenderHelpItem("tab", "→apps"),
				ui.RenderHelpItem("?", "help"),
			}
		} else {
			items = []string{
				ui.RenderHelpItem("space", "select"),
				ui.RenderHelpItem("v", "preview"),
				ui.RenderHelpItem("d", "diff"),
				ui.RenderHelpItem("e", "edit"),
				ui.RenderHelpItem("tab", "→apps"),
				ui.RenderHelpItem("?", "help"),
			}
		}
	}

	return ui.HelpBarStyle.Render(strings.Join(items, "  "))
}

func (m *Model) renderHelp() string {
	var b strings.Builder

	b.WriteString(ui.PanelTitleStyle.Render("⌨️  Keyboard Shortcuts Guide"))
	b.WriteString("\n\n")

	// Quick Actions section (most important - at the top)
	b.WriteString(ui.MutedStyle.Render("  ─── ⚡ Quick Actions ───"))
	b.WriteString("\n")
	quickBindings := []struct {
		key  string
		desc string
	}{
		{"Q", "Quick Backup: auto-backup files to dotfiles"},
		{"P", "Push + Commit: push selected + git commit"},
		{"p", "Push: copy local → dotfiles (manual)"},
		{"l", "Pull: copy dotfiles → local"},
		{"c", "Check conflicts"},
		{"e", "Open in editor (VS Code/Cursor/Zed)"},
	}
	for _, bind := range quickBindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ui.HelpKeyStyle.Width(14).Render(bind.key),
			ui.HelpDescStyle.Render(bind.desc),
		))
	}

	// Mode section - More detailed explanation
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render("  ─── 💾 Backup vs Sync ───"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		ui.HelpKeyStyle.Width(14).Render("[B] Backup"),
		ui.HelpDescStyle.Render("Lưu riêng theo máy → Q tự động push"),
	))
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		ui.HelpKeyStyle.Width(14).Render("[B+S] Sync"),
		ui.HelpDescStyle.Render("Giống nhau mọi máy → p/l thủ công"),
	))
	b.WriteString("\n")
	modeBindings := []struct {
		key  string
		desc string
	}{
		{"t", "Toggle sync cho app/file đang chọn"},
		{"R", "Restore config từ máy khác"},
	}
	for _, bind := range modeBindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ui.HelpKeyStyle.Width(14).Render(bind.key),
			ui.HelpDescStyle.Render(bind.desc),
		))
	}

	// Navigation section
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render("  ─── 🧭 Navigation ───"))
	b.WriteString("\n")
	navBindings := []struct {
		key  string
		desc string
	}{
		{"/", "Search/filter apps"},
		{"1-9", "Filter by category"},
		{"0", "Clear category filter"},
		{"↑/k ↓/j", "Move cursor up/down"},
		{"Tab", "Switch Apps ↔ Files panel"},
		{"PgUp/PgDn", "Scroll page"},
		{"Home/End", "Jump to first/last"},
	}
	for _, bind := range navBindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ui.HelpKeyStyle.Width(14).Render(bind.key),
			ui.HelpDescStyle.Render(bind.desc),
		))
	}

	// Selection section
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render("  ─── ✅ Selection ───"))
	b.WriteString("\n")
	selBindings := []struct {
		key  string
		desc string
	}{
		{"Space", "Toggle selection"},
		{"a", "Select all"},
		{"D", "Deselect all"},
		{"M", "Select all modified (need push)"},
		{"O", "Select all outdated (need pull)"},
		{"u", "Undo last selection"},
	}
	for _, bind := range selBindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ui.HelpKeyStyle.Width(14).Render(bind.key),
			ui.HelpDescStyle.Render(bind.desc),
		))
	}

	// File Actions section
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render("  ─── 📄 File Actions ───"))
	b.WriteString("\n")
	fileBindings := []struct {
		key  string
		desc string
	}{
		{"v/Enter", "Preview file content"},
		{"d", "View diff (local vs dotfiles)"},
		{"m", "Merge conflicts"},
		{"s", "Rescan all apps"},
		{"b", "Export Brewfile"},
		{"r", "Refresh current view"},
	}
	for _, bind := range fileBindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ui.HelpKeyStyle.Width(14).Render(bind.key),
			ui.HelpDescStyle.Render(bind.desc),
		))
	}

	// Git Operations section
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render("  ─── 🔀 Git (press 'g') ───"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s\n", ui.HelpDescStyle.Render("Tự động tạo git nếu chưa có")))
	gitBindings := []struct {
		key  string
		desc string
	}{
		{"g", "Open git panel (auto git init)"},
		{"a", "Stage all"},
		{"c", "Commit"},
		{"p", "Push"},
		{"f", "Fetch"},
		{"l", "Pull"},
		{"b", "Switch branch"},
	}
	for _, bind := range gitBindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ui.HelpKeyStyle.Width(14).Render(bind.key),
			ui.HelpDescStyle.Render(bind.desc),
		))
	}

	// General section
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render("  ─── ⚙️ General ───"))
	b.WriteString("\n")
	generalBindings := []struct {
		key  string
		desc string
	}{
		{",", "Settings (dotfiles path, backup path)"},
		{"?", "Toggle this help"},
		{"Esc", "Go back / Cancel"},
		{"q", "Quit"},
	}
	for _, bind := range generalBindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ui.HelpKeyStyle.Width(14).Render(bind.key),
			ui.HelpDescStyle.Render(bind.desc),
		))
	}

	// Status icons legend
	b.WriteString("\n")
	b.WriteString(ui.PanelTitleStyle.Render("📊 Status Icons"))
	b.WriteString("\n\n")
	statusIcons := []struct {
		icon string
		desc string
	}{
		{"✓", "Synced - Files are identical"},
		{"●", "Modified - Local has changes (push)"},
		{"○", "Outdated - Dotfiles has updates (pull)"},
		{"⚡", "Conflict - Both sides changed"},
		{"[S]", "Sync mode - Same on all machines"},
		{"[B]", "Backup mode - Per-machine storage"},
	}
	for _, icon := range statusIcons {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ui.HelpKeyStyle.Width(4).Render(icon.icon),
			ui.HelpDescStyle.Render(icon.desc),
		))
	}

	// Quick reference - Backup explanation
	b.WriteString("\n")
	b.WriteString(ui.PanelTitleStyle.Render("💡 Cách hoạt động"))
	b.WriteString("\n\n")
	b.WriteString(ui.MutedStyle.Render("  Backup mode [B]:"))
	b.WriteString("\n")
	b.WriteString("    • Mỗi máy có folder riêng: dotfiles/app/{machine}/\n")
	b.WriteString("    • Nhấn Q → tự động push lên folder của máy này\n")
	b.WriteString("    • Dùng R để restore config từ máy khác\n")
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render("  Sync mode [S]:"))
	b.WriteString("\n")
	b.WriteString("    • Một bản duy nhất: dotfiles/app/file\n")
	b.WriteString("    • Nhấn p để push, l để pull (thủ công)\n")
	b.WriteString("    • Giống nhau trên mọi máy\n")
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render("  Press any key to close"))

	return b.String()
}

func (m *Model) renderDiff() string {
	var b strings.Builder

	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Render diff view
	b.WriteString(m.diffView.View())

	return ui.AppStyle.Render(b.String())
}

func (m *Model) renderMerge() string {
	var b strings.Builder

	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Render merge view
	b.WriteString(m.mergeView.View())

	return ui.AppStyle.Render(b.String())
}

func (m *Model) renderPreview() string {
	var b strings.Builder

	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Render file preview
	b.WriteString(m.filePreview.View())
	b.WriteString("\n")

	// Help bar
	helpItems := []string{
		ui.RenderHelpItem("j/k", "scroll"),
		ui.RenderHelpItem("PgUp/Dn", "page"),
		ui.RenderHelpItem("Home/End", "top/bottom"),
		ui.RenderHelpItem("q/Esc", "close"),
	}
	b.WriteString(ui.HelpBarStyle.Render(strings.Join(helpItems, "  ")))

	return ui.AppStyle.Render(b.String())
}

func (m *Model) renderGit() string {
	var b strings.Builder

	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Render git panel
	b.WriteString(m.gitPanel.View())

	return ui.AppStyle.Render(b.String())
}

func (m *Model) renderSettings() string {
	width := 70
	style := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Primary)

	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Primary).
		Render("⚙️  Settings")

	b.WriteString(title)
	b.WriteString("\n\n")

	// Settings fields
	fields := []struct {
		name  string
		value string
		field SettingsField
	}{
		{"Dotfiles Path", m.config.DotfilesPath, SettingsDotfilesPath},
		{"Backup Path", m.config.BackupPath, SettingsBackupPath},
	}

	for _, f := range fields {
		isSelected := m.settingsField == f.field

		// Label
		labelStyle := lipgloss.NewStyle().Width(15)
		if isSelected {
			labelStyle = labelStyle.Bold(true).Foreground(ui.Primary)
		} else {
			labelStyle = labelStyle.Foreground(lipgloss.Color("#6c7086"))
		}
		b.WriteString(labelStyle.Render(f.name + ":"))
		b.WriteString(" ")

		// Value or input
		if isSelected && m.settingsEditing {
			// Show text input
			b.WriteString(m.textInput.View())
		} else {
			valueStyle := lipgloss.NewStyle()
			if isSelected {
				valueStyle = valueStyle.
					Background(lipgloss.Color("#313244")).
					Foreground(lipgloss.Color("#cdd6f4")).
					Padding(0, 1)
			} else {
				valueStyle = valueStyle.Foreground(lipgloss.Color("#cdd6f4"))
			}
			b.WriteString(valueStyle.Render(f.value))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	if m.settingsEditing {
		b.WriteString(helpStyle.Render("Enter: save  •  Esc: cancel"))
	} else {
		b.WriteString(helpStyle.Render("↑/↓: navigate  •  Enter: edit  •  Esc/q: back"))
	}

	// Current config file path
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Config file: " + config.ConfigPath()))

	box := style.Render(b.String())

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func (m *Model) handleGitKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle branch mode separately
	if m.gitPanel.Mode == components.ModeBranches {
		return m.handleGitBranchKeys(msg)
	}

	switch msg.String() {
	case "esc", "q":
		m.screen = ScreenMain
		m.status = "Ready"
		return m, nil

	case "a":
		// Add all changes
		if err := m.gitPanel.AddAll(); err != nil {
			m.status = fmt.Sprintf("Add failed: %v", err)
		} else {
			m.status = "All changes staged"
		}
		return m, nil

	case "c":
		// Open commit message dialog
		if !m.gitPanel.HasStagedChanges() {
			m.status = "No staged changes to commit"
			return m, nil
		}
		// Reset textarea for commit message
		m.textArea.Reset()
		m.textArea.Placeholder = "Enter commit message..."
		m.textArea.Focus()
		m.screen = ScreenCommit
		return m, textarea.Blink

	case "p":
		// Push
		if err := m.gitPanel.Push(); err != nil {
			m.status = fmt.Sprintf("Push failed: %v", err)
		} else {
			m.status = "Pushed successfully"
		}
		return m, nil

	case "f":
		// Fetch
		if err := m.gitPanel.Fetch(); err != nil {
			m.status = fmt.Sprintf("Fetch failed: %v", err)
		} else {
			m.status = "Fetched from remote"
		}
		return m, nil

	case "l":
		// Pull
		if err := m.gitPanel.Pull(); err != nil {
			m.status = fmt.Sprintf("Pull failed: %v", err)
		} else {
			m.status = "Pulled from remote"
		}
		return m, nil

	case "r":
		// Refresh
		m.gitPanel.Refresh()
		m.status = "Git status refreshed"
		return m, nil

	case "s":
		// Stash
		if err := m.gitPanel.Stash(); err != nil {
			m.status = fmt.Sprintf("Stash failed: %v", err)
		} else {
			m.status = "Changes stashed"
		}
		return m, nil

	case "S":
		// Stash pop
		if err := m.gitPanel.StashPop(); err != nil {
			m.status = fmt.Sprintf("Stash pop failed: %v", err)
		} else {
			m.status = "Stash popped"
		}
		return m, nil

	case "b":
		// Toggle branch mode
		m.gitPanel.ToggleBranchMode()
		if m.gitPanel.Mode == components.ModeBranches {
			m.status = "Select branch to checkout"
		} else {
			m.status = "Git status"
		}
		return m, nil

	case "j", "down":
		m.gitPanel.MoveDown()
		return m, nil

	case "k", "up":
		m.gitPanel.MoveUp()
		return m, nil
	}

	return m, nil
}

// handleGitBranchKeys handles keys in branch selection mode
func (m *Model) handleGitBranchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "b":
		// Exit branch mode
		m.gitPanel.Mode = components.ModeStatus
		m.status = "Git status"
		return m, nil

	case "j", "down":
		m.gitPanel.MoveBranchDown()
		return m, nil

	case "k", "up":
		m.gitPanel.MoveBranchUp()
		return m, nil

	case "enter":
		// Checkout selected branch
		branch := m.gitPanel.GetSelectedBranch()
		if branch == "" {
			m.status = "No branch selected"
			return m, nil
		}
		if err := m.gitPanel.CheckoutBranch(); err != nil {
			m.status = fmt.Sprintf("Checkout failed: %v", err)
		} else {
			m.status = fmt.Sprintf("Switched to branch: %s", branch)
		}
		return m, nil
	}

	return m, nil
}

// handleCommitKeys handles keys in the commit message dialog
func (m *Model) handleCommitKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel commit
		m.screen = ScreenGit
		m.textArea.Blur()
		m.status = "Commit cancelled"
		return m, nil

	case tea.KeyCtrlS:
		// Ctrl+S to commit (since Enter is used for newline in textarea)
		message := strings.TrimSpace(m.textArea.Value())
		if message == "" {
			m.status = "Commit message cannot be empty"
			return m, nil
		}
		if err := m.gitPanel.Commit(message); err != nil {
			m.status = fmt.Sprintf("Commit failed: %v", err)
		} else {
			m.status = "Committed! Press 'p' to push to remote"
			// Show a prompt to push after successful commit
			m.gitPanel.Refresh()
		}
		m.textArea.Blur()
		m.textArea.Reset()
		m.screen = ScreenGit
		return m, nil
	}

	// Pass other keys to textarea
	var cmd tea.Cmd
	m.textArea, cmd = m.textArea.Update(msg)
	return m, cmd
}

// renderCommitDialog renders the commit message input dialog
func (m *Model) renderCommitDialog() string {
	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Dialog box
	width := 60
	style := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Primary)

	var content strings.Builder
	content.WriteString(ui.PanelTitleStyle.Render("📝 Commit Changes"))
	content.WriteString("\n\n")

	// Show staged files count
	stagedCount := 0
	if m.gitPanel.Status != nil {
		stagedCount = len(m.gitPanel.Status.Staged)
	}
	content.WriteString(fmt.Sprintf("Files to commit: %d\n\n", stagedCount))

	// Input field - using textarea for multi-line messages
	content.WriteString("Commit message:\n")
	content.WriteString(m.textArea.View())
	content.WriteString("\n\n")

	// Help text
	content.WriteString(ui.MutedStyle.Render("Ctrl+S to commit • ESC to cancel"))

	box := style.Render(content.String())

	// Center the box
	b.WriteString(box)

	return ui.AppStyle.Render(b.String())
}

// handleSearchKeys handles key input in search mode
func (m *Model) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel search, restore original app list
		m.searchMode = false
		m.searchQuery = ""
		m.textInput.Blur()
		m.appList.SetApps(m.apps)
		m.filteredApps = nil
		m.status = "Search cancelled"
		m.updateFileList()
		return m, nil

	case tea.KeyEnter:
		// Confirm search
		m.searchMode = false
		m.textInput.Blur()
		if m.searchQuery == "" {
			m.appList.SetApps(m.apps)
			m.filteredApps = nil
			m.status = fmt.Sprintf("Showing all %d apps", len(m.apps))
		} else {
			m.status = fmt.Sprintf("Showing %d matching apps", len(m.filteredApps))
		}
		m.updateFileList()
		return m, nil

	case tea.KeyBackspace, tea.KeyDelete:
		// Handle backspace in textinput
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		m.searchQuery = m.textInput.Value()
		m.filterApps()
		return m, cmd

	case tea.KeyUp:
		// Navigate up in filtered results
		m.appList.MoveUp()
		m.updateFileList()
		return m, nil

	case tea.KeyDown:
		// Navigate down in filtered results
		m.appList.MoveDown()
		m.updateFileList()
		return m, nil

	default:
		// Handle regular typing
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		m.searchQuery = m.textInput.Value()
		m.filterApps()
		return m, cmd
	}
}

// filterApps filters the app list based on search query
func (m *Model) filterApps() {
	if m.searchQuery == "" {
		m.appList.SetApps(m.apps)
		m.filteredApps = nil
		m.status = fmt.Sprintf("Type to search (%d apps)", len(m.apps))
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []*models.App

	for _, app := range m.apps {
		// Match against app name, ID, or category
		nameLower := strings.ToLower(app.Name)
		idLower := strings.ToLower(app.ID)
		categoryLower := strings.ToLower(app.Category)

		if strings.Contains(nameLower, query) ||
			strings.Contains(idLower, query) ||
			strings.Contains(categoryLower, query) {
			filtered = append(filtered, app)
		}
	}

	m.filteredApps = filtered
	m.appList.SetApps(filtered)
	m.status = fmt.Sprintf("Found %d apps matching '%s'", len(filtered), m.searchQuery)
}

// filterByCategory filters apps by category
func (m *Model) filterByCategory(category string) (tea.Model, tea.Cmd) {
	if m.categoryFilter == category {
		// Toggle off if same category
		return m.clearCategoryFilter()
	}

	m.categoryFilter = category
	var filtered []*models.App

	for _, app := range m.apps {
		if strings.ToLower(app.Category) == category {
			filtered = append(filtered, app)
		}
	}

	m.filteredApps = filtered
	m.appList.SetApps(filtered)
	m.updateFileList()

	categoryLabels := map[string]string{
		"ai":           "AI Tools",
		"shell":        "Shells",
		"editor":       "Editors",
		"terminal":     "Terminals",
		"git":          "Git Tools",
		"dev":          "Dev Tools",
		"cli":          "CLI Tools",
		"productivity": "Productivity",
		"cloud":        "Cloud/Infra",
	}

	label := categoryLabels[category]
	if label == "" {
		label = category
	}
	m.status = fmt.Sprintf("Filtered: %s (%d apps) • Press 0 to clear", label, len(filtered))
	return m, nil
}

// clearCategoryFilter clears the category filter
func (m *Model) clearCategoryFilter() (tea.Model, tea.Cmd) {
	m.categoryFilter = ""
	m.searchQuery = ""
	m.filteredApps = nil
	m.appList.SetApps(m.apps)
	m.updateFileList()
	m.status = fmt.Sprintf("Showing all %d apps", len(m.apps))
	return m, nil
}

// clearAllFilters clears both search and category filters
func (m *Model) clearAllFilters() (tea.Model, tea.Cmd) {
	return m.clearCategoryFilter()
}

// handleSelectModified selects all apps/files with modifications
func (m *Model) handleSelectModified() (tea.Model, tea.Cmd) {
	m.saveSelectionState() // Save before changing
	modifiedCount := 0

	if m.focusedPanel == PanelApps {
		// Select all apps that have modified or conflicting files
		for _, app := range m.apps {
			hasModified := false
			for _, file := range app.Files {
				switch file.ConflictType {
				case models.ConflictLocalModified, models.ConflictLocalNew,
					models.ConflictDotfilesModified, models.ConflictDotfilesNew,
					models.ConflictBothModified:
					hasModified = true
					break
				}
				if hasModified {
					break
				}
			}
			if hasModified {
				app.Selected = true
				modifiedCount++
			}
		}
		m.appList.SetApps(m.apps)
		m.status = fmt.Sprintf("Selected %d apps with modifications", modifiedCount)
	} else {
		// Select all files that have modifications in current file list
		for i := range m.fileList.Files {
			switch m.fileList.Files[i].ConflictType {
			case models.ConflictLocalModified, models.ConflictLocalNew,
				models.ConflictDotfilesModified, models.ConflictDotfilesNew,
				models.ConflictBothModified:
				m.fileList.Files[i].Selected = true
				modifiedCount++
			}
		}
		m.syncFilesToApp()
		m.status = fmt.Sprintf("Selected %d modified files", modifiedCount)
	}

	return m, nil
}

// handleSelectOutdated selects all apps/files that need to be pulled (outdated)
func (m *Model) handleSelectOutdated() (tea.Model, tea.Cmd) {
	m.saveSelectionState() // Save before changing
	outdatedCount := 0

	if m.focusedPanel == PanelApps {
		// Select all apps that have outdated files (need pull)
		for _, app := range m.apps {
			hasOutdated := false
			for _, file := range app.Files {
				switch file.ConflictType {
				case models.ConflictDotfilesModified, models.ConflictDotfilesNew:
					hasOutdated = true
					break
				}
				if hasOutdated {
					break
				}
			}
			if hasOutdated {
				app.Selected = true
				outdatedCount++
			}
		}
		m.appList.SetApps(m.apps)
		m.status = fmt.Sprintf("Selected %d apps with outdated files (need pull)", outdatedCount)
	} else {
		// Select all files that are outdated in current file list
		for i := range m.fileList.Files {
			switch m.fileList.Files[i].ConflictType {
			case models.ConflictDotfilesModified, models.ConflictDotfilesNew:
				m.fileList.Files[i].Selected = true
				outdatedCount++
			}
		}
		m.syncFilesToApp()
		m.status = fmt.Sprintf("Selected %d outdated files (need pull)", outdatedCount)
	}

	return m, nil
}

// handleRefresh refreshes the current view by rescanning
func (m *Model) handleRefresh() (tea.Model, tea.Cmd) {
	// If a category filter is active, preserve it after refresh
	savedFilter := m.categoryFilter

	m.screen = ScreenScanning
	m.status = "Refreshing..."

	// Create a wrapped scan function that restores filter after scan
	return m, func() tea.Msg {
		s := scanner.New(m.config.AppsConfig)
		apps, err := s.Scan()

		for _, app := range apps {
			sync.UpdateSyncStatusWithHashes(app, m.config.DotfilesPath, m.stateManager)
		}

		// Restore category filter state in the message
		return refreshCompleteMsg{
			apps:           apps,
			err:            err,
			categoryFilter: savedFilter,
		}
	}
}

// saveSelectionState saves the current selection state for undo
func (m *Model) saveSelectionState() {
	m.lastAppSelections = make(map[string]bool)
	m.lastFileSelections = make(map[string]bool)

	for _, app := range m.apps {
		m.lastAppSelections[app.ID] = app.Selected
		for _, file := range app.Files {
			m.lastFileSelections[file.Path] = file.Selected
		}
	}
	m.canUndo = true
}

// handleUndo restores the previous selection state
func (m *Model) handleUndo() (tea.Model, tea.Cmd) {
	if !m.canUndo || m.lastAppSelections == nil {
		m.status = "Nothing to undo"
		return m, nil
	}

	// Restore app selections
	for _, app := range m.apps {
		if selected, ok := m.lastAppSelections[app.ID]; ok {
			app.Selected = selected
		}
		// Restore file selections
		for i := range app.Files {
			if selected, ok := m.lastFileSelections[app.Files[i].Path]; ok {
				app.Files[i].Selected = selected
			}
		}
	}

	m.appList.SetApps(m.apps)
	m.updateFileList()
	m.canUndo = false
	m.status = "Selection restored"
	return m, nil
}

// handleQuickSync runs the Quick Sync workflow
func (m *Model) handleQuickSync() (tea.Model, tea.Cmd) {
	if m.quickSync == nil {
		m.status = "Quick backup not initialized"
		return m, nil
	}

	selectedApps := m.appList.SelectedApps()
	if len(selectedApps) == 0 {
		m.status = "No apps selected"
		return m, nil
	}

	m.status = "Running quick backup..."
	m.syncing = true

	return m, func() tea.Msg {
		result := m.quickSync.Run(selectedApps)
		return quickSyncCompleteMsg{result: result}
	}
}

// quickSyncCompleteMsg is sent when quick sync completes
type quickSyncCompleteMsg struct {
	result *quicksync.Result
}

// handleToggleMode toggles the mode for the selected app/file
func (m *Model) handleToggleMode() (tea.Model, tea.Cmd) {
	if m.modesConfig == nil {
		m.status = "Modes not initialized"
		return m, nil
	}

	if m.focusedPanel == PanelApps {
		// Toggle app sync
		currentApp := m.appList.Current()
		if currentApp == nil {
			m.status = "No app selected"
			return m, nil
		}

		synced := m.modesConfig.ToggleAppSync(currentApp.ID)
		if err := m.modesConfig.Save(); err != nil {
			m.status = fmt.Sprintf("Failed to save mode: %v", err)
			return m, nil
		}

		if synced {
			m.status = fmt.Sprintf("%s: sync enabled", currentApp.Name)
		} else {
			m.status = fmt.Sprintf("%s: sync disabled", currentApp.Name)
		}
		m.appList.SetModesConfig(m.modesConfig)
		m.updateFileList()
	} else {
		// Toggle file sync
		currentApp := m.appList.Current()
		currentFile := m.fileList.Current()
		if currentApp == nil || currentFile == nil {
			m.status = "No file selected"
			return m, nil
		}

		synced := m.modesConfig.ToggleFileSync(currentApp.ID, currentFile.Path)
		if err := m.modesConfig.Save(); err != nil {
			m.status = fmt.Sprintf("Failed to save mode: %v", err)
			return m, nil
		}

		if synced {
			m.status = fmt.Sprintf("%s: sync enabled", currentFile.Name)
		} else {
			m.status = fmt.Sprintf("%s: sync disabled", currentFile.Name)
		}
		m.fileList.SetModesConfig(m.modesConfig)
	}

	return m, nil
}

// handleRestore opens the restore from machine dialog
func (m *Model) handleRestore() (tea.Model, tea.Cmd) {
	if m.backupManager == nil {
		m.status = "Backup manager not initialized"
		return m, nil
	}

	// Load available machines
	machines, err := m.backupManager.ListMachines()
	if err != nil {
		m.status = fmt.Sprintf("Failed to list machines: %v", err)
		return m, nil
	}

	if len(machines) == 0 {
		m.status = "No backup machines found"
		return m, nil
	}

	m.restoreMachines = machines
	m.restoreCursor = 0
	m.status = "Select machine to restore from"
	// TODO: Switch to restore screen when implemented
	m.status = fmt.Sprintf("Found %d machines with backups. Restore screen coming soon.", len(machines))
	return m, nil
}

// handleCheckConflicts runs conflict detection and displays results
func (m *Model) handleCheckConflicts() (tea.Model, tea.Cmd) {
	if m.quickSync == nil {
		m.status = "Quick backup not initialized"
		return m, nil
	}

	selectedApps := m.appList.SelectedApps()
	if len(selectedApps) == 0 {
		selectedApps = m.apps
	}

	m.status = "Checking for conflicts..."

	return m, func() tea.Msg {
		detection := m.quickSync.DetectOnly(selectedApps)
		return conflictCheckMsg{detection: detection}
	}
}

// conflictCheckMsg is sent when conflict check completes
type conflictCheckMsg struct {
	detection *quicksync.DetectionResult
}

// handleOpenEditor opens the current file in the configured editor
func (m *Model) handleOpenEditor() (tea.Model, tea.Cmd) {
	if m.focusedPanel != PanelFiles {
		m.status = "Select a file first (Tab to switch panel)"
		return m, nil
	}

	currentFile := m.fileList.Current()
	if currentFile == nil {
		m.status = "No file selected"
		return m, nil
	}

	// Detect and open editor
	ed, err := editor.Detect(nil)
	if err != nil {
		m.status = fmt.Sprintf("No editor found: %v", err)
		return m, nil
	}

	m.status = fmt.Sprintf("Opening %s in %s...", currentFile.Name, ed.Name())

	return m, func() tea.Msg {
		err := ed.OpenDiff(currentFile.Path, currentFile.Path)
		return editorOpenedMsg{err: err}
	}
}

// editorOpenedMsg is sent when editor operation completes
type editorOpenedMsg struct {
	err error
}

// handlePushAndCommit pushes changes and commits with auto-generated message
func (m *Model) handlePushAndCommit() (tea.Model, tea.Cmd) {
	selectedApps := m.appList.SelectedApps()
	if len(selectedApps) == 0 {
		m.status = "No apps selected"
		return m, nil
	}

	// Count selected files
	fileCount := 0
	var appNames []string
	for _, app := range selectedApps {
		hasFiles := false
		for _, file := range app.Files {
			if file.Selected {
				fileCount++
				hasFiles = true
			}
		}
		if hasFiles {
			appNames = append(appNames, app.ID)
		}
	}

	if fileCount == 0 {
		m.status = "No files selected"
		return m, nil
	}

	m.status = "Pushing and committing..."
	m.syncing = true

	return m, func() tea.Msg {
		// Export files first
		exporter := sync.NewExporter(m.config)
		results, err := exporter.ExportAll(selectedApps)
		if err != nil {
			return syncCompleteMsg{err: err, action: "push"}
		}

		// Generate commit message
		var commitMsg string
		if len(appNames) == 1 {
			commitMsg = fmt.Sprintf("sync: update %s (%d files)", appNames[0], fileCount)
		} else if len(appNames) <= 3 {
			commitMsg = fmt.Sprintf("sync: update %s", strings.Join(appNames, ", "))
		} else {
			commitMsg = fmt.Sprintf("sync: update %d apps (%d files)", len(appNames), fileCount)
		}

		// Commit and push
		gitRepo := git.NewRepo(m.config.DotfilesPath)
		if gitRepo.IsRepo() {
			_ = gitRepo.AddAll()
			_ = gitRepo.Commit(commitMsg)
			if gitRepo.HasRemote() {
				_ = gitRepo.Push()
			}
		}

		return syncCompleteMsg{results: results, action: "push+commit"}
	}
}

func main() {
	// Check for flags
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-v", "--version", "version":
			fmt.Printf("dotsync %s (built %s)\n", version, buildTime)
			return
		case "-h", "--help", "help":
			fmt.Println("dotsync - A beautiful TUI for managing dotfiles")
			fmt.Println()
			fmt.Println("Usage: dotsync [options]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  -v, --version    Show version")
			fmt.Println("  -h, --help       Show this help")
			fmt.Println("  -d, --debug      Enable debug mode (logs to stderr)")
			fmt.Println()
			fmt.Println("Run without arguments to start the TUI.")
			return
		case "-d", "--debug", "debug":
			debugMode = true
			scanner.DebugMode = true
			fmt.Fprintln(os.Stderr, "[DEBUG] Debug mode enabled")
		}
	}

	p := tea.NewProgram(New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

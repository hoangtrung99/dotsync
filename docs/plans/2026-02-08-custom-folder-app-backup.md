# Custom Folder/App Backup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an in-app flow to register custom folders/apps so they appear in the Apps panel and can be backed up with existing Push/Quick Backup flows.

**Architecture:** Keep scanner as the single source of truth for discovered apps, but extend it to merge built-in definitions with user-defined custom definitions persisted on disk. Add a lightweight TUI form to create custom entries and trigger a rescan. Reuse existing sync/backup pipelines so custom items do not need a separate backup engine.

**Tech Stack:** Go 1.24+, Bubble Tea/Bubbles/Lip Gloss, YAML (`gopkg.in/yaml.v3`), existing `internal/scanner`, `internal/models`, `main.go` event loop.

---

## Design Decision (Brainstorming Summary)

**Recommended option (A):** Add a new `Add Custom Source` flow in UI, persist to `~/.config/dotsync/apps.yaml`, scanner merges built-ins + custom definitions.
- Pros: Fast user flow, no manual file edits, fully compatible with current scanner/sync architecture.
- Cons: `main.go` state grows; needs careful input validation.

**Option B:** Only document/edit `apps.yaml` manually (no UI).
- Pros: Small code delta.
- Cons: Does not satisfy “add from Apps section” UX requirement.

**Option C:** Session-only custom entries (not persisted).
- Pros: Simplest runtime implementation.
- Cons: Poor UX; entries disappear after restart.

We implement **Option A**.

## Execution Guardrails

- Follow `@superpowers:test-driven-development` for each task before implementation.
- Before declaring completion, run `@superpowers:verification-before-completion`.
- Keep changes YAGNI: add “create custom entry” only (no edit/delete yet).

### Task 1: Add Custom App Definition Store

**Files:**
- Create: `internal/customapps/store.go`
- Create: `internal/customapps/store_test.go`
- Modify: `internal/models/app.go`
- Test: `internal/customapps/store_test.go`

**Step 1: Write failing tests for custom definition persistence**

```go
func TestStore_LoadMissingFile_ReturnsEmpty(t *testing.T) {}
func TestStore_AddFolderEntry_PersistsAsAppDefinition(t *testing.T) {}
func TestStore_AddAppEntry_WithMultiplePaths(t *testing.T) {}
func TestStore_AddDuplicateID_ReturnsError(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/customapps -v`
Expected: FAIL with missing package/types/functions.

**Step 3: Write minimal implementation**

```go
type EntryType string
const (
    EntryTypeFolder EntryType = "folder"
    EntryTypeApp    EntryType = "app"
)

type Entry struct {
    ID, Name, Category, Icon string
    ConfigPaths []string
}

type Store struct { path string }
func New(path string) *Store
func (s *Store) Load() ([]models.AppDefinition, error)
func (s *Store) Add(def models.AppDefinition) error
```

Rules:
- Persist as YAML root: `apps: []`.
- Normalize path `~/...` and absolute paths.
- Validate required fields: `id`, `name`, `config_paths`.
- Reject duplicate IDs inside custom file.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/customapps -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/customapps/store.go internal/customapps/store_test.go internal/models/app.go
git commit -m "feat: add custom app definition store"
```

### Task 2: Merge Built-in + Custom Definitions in Scanner

**Files:**
- Modify: `internal/scanner/scanner.go`
- Modify: `internal/scanner/scanner_test.go`
- Test: `internal/scanner/scanner_test.go`

**Step 1: Write failing tests for merge behavior**

```go
func TestScan_MergesBuiltinAndCustomDefinitions(t *testing.T) {}
func TestScan_UsesDefaultCustomConfigWhenAppsConfigEmpty(t *testing.T) {}
func TestScan_CustomDefinitionOverridesBuiltinOnSameID(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/scanner -run "TestScan_MergesBuiltinAndCustomDefinitions|TestScan_UsesDefaultCustomConfigWhenAppsConfigEmpty|TestScan_CustomDefinitionOverridesBuiltinOnSameID" -v`
Expected: FAIL due to current scanner using either built-ins or external file, not merge.

**Step 3: Write minimal implementation**

```go
func (s *Scanner) definitionsPath() string
func (s *Scanner) loadCustomDefinitions() ([]models.AppDefinition, error)
func mergeDefinitions(builtin, custom []models.AppDefinition) []models.AppDefinition
```

Behavior:
- Always start from `getBuiltinDefinitions()`.
- Load custom definitions from:
  - `s.configPath` if set.
  - Else default `~/.config/dotsync/apps.yaml`.
- Missing custom file is non-fatal.
- Merge by `id`; custom overrides same-id built-in, and appends new IDs.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/scanner -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/scanner/scanner.go internal/scanner/scanner_test.go
git commit -m "feat: merge custom app definitions with builtin scanner defs"
```

### Task 3: Add Custom Source Form Model (Testable, UI-agnostic)

**Files:**
- Create: `internal/customapps/form.go`
- Create: `internal/customapps/form_test.go`
- Test: `internal/customapps/form_test.go`

**Step 1: Write failing tests for form validation/parsing**

```go
func TestBuildDefinition_FolderMode(t *testing.T) {}
func TestBuildDefinition_AppModeMultiplePaths(t *testing.T) {}
func TestBuildDefinition_RejectsEmptyName(t *testing.T) {}
func TestBuildDefinition_RejectsMissingPath(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/customapps -run "TestBuildDefinition_" -v`
Expected: FAIL with undefined builder/validation logic.

**Step 3: Write minimal implementation**

```go
type FormInput struct {
    Mode string // "folder" | "app"
    Name string
    Paths []string
}

func BuildDefinition(in FormInput) (models.AppDefinition, error)
```

Rules:
- Folder mode: exactly 1 path.
- App mode: 1..N paths.
- Auto-generate stable ID slug from `Name`.
- Default category `custom`, default icon `📁`.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/customapps -run "TestBuildDefinition_" -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/customapps/form.go internal/customapps/form_test.go
git commit -m "feat: add custom source form validation and definition builder"
```

### Task 4: Integrate Add Custom Source Flow into Main TUI

**Files:**
- Modify: `main.go`
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/keys_test.go`
- Test: `internal/ui/keys_test.go`

**Step 1: Write failing tests for keybindings/help exposure**

```go
func TestDefaultKeyMap_HasAddCustomBinding(t *testing.T) {}
func TestFullHelp_IncludesAddCustomBinding(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ui -run "TestDefaultKeyMap_HasAddCustomBinding|TestFullHelp_IncludesAddCustomBinding" -v`
Expected: FAIL because keymap has no add-custom action yet.

**Step 3: Write minimal implementation**

```go
// keys.go
AddCustom key.Binding // "+" add custom app/folder

// main.go
const ScreenAddCustom Screen = ...
func (m *Model) handleAddCustom() (tea.Model, tea.Cmd)
func (m *Model) handleAddCustomKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd)
func (m *Model) renderAddCustom() string
```

Flow:
- In Apps panel, press `+`.
- Form fields: Mode (`folder`/`app`), Name, Path(s).
- On submit: `customapps.BuildDefinition` -> `customapps.Store.Add` -> trigger `scanApps`.
- Show status: success/error with actionable message.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ui -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add main.go internal/ui/keys.go internal/ui/keys_test.go
git commit -m "feat: add custom source creation flow in apps panel"
```

### Task 5: Update Help, Settings Copy, and README for Custom Source UX

**Files:**
- Modify: `main.go`
- Modify: `README.md`
- Modify: `CONTRIBUTING.md`
- Test: N/A (docs + UI text)

**Step 1: Write failing text assertions where feasible**

Add focused UI text checks in existing tests if available (`internal/ui/keys_test.go`) for help label.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ui -run "Help|KeyMap" -v`
Expected: FAIL until help entries are updated.

**Step 3: Write minimal implementation**

Update:
- Help legend to include `+ Add custom source`.
- README custom app section to correct schema (`config_paths`, not `paths`).
- Add examples for both folder and app entries.
- CONTRIBUTING notes on validating custom paths and IDs.

**Step 4: Run tests and lint-like checks**

Run:
- `go test ./internal/ui -v`
- `go test ./internal/scanner ./internal/customapps -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add main.go README.md CONTRIBUTING.md internal/ui/keys_test.go
git commit -m "docs: add custom folder/app backup usage and help updates"
```

### Task 6: Full Verification Before Completion

**Files:**
- Verify whole repo (no additional file changes unless needed)

**Step 1: Run targeted package tests**

Run:
- `go test ./internal/customapps ./internal/scanner ./internal/ui ./internal/ui/components -v`

Expected: PASS.

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS.

**Step 3: Manual smoke test**

Run:
- `go run .`
- Press `+` in Apps panel, add folder `~/.hammerspoon`
- Confirm app appears and can be selected
- Press `Q` or `p` and verify files written to dotfiles repo

Expected:
- New custom item is visible after rescan.
- Backup/sync paths follow current mode rules.

**Step 4: Validate persisted config**

Run:
- `cat ~/.config/dotsync/apps.yaml`

Expected:
- New `apps:` entry exists with `id`, `name`, `category`, `icon`, `config_paths`.

**Step 5: Final commit**

```bash
git add -A
git commit -m "feat: support adding custom folder/app from apps panel for backup"
```

## Risks and Mitigations

- **Risk:** Invalid custom paths break scan UX.
  - **Mitigation:** Pre-submit path validation + clear error in status bar.
- **Risk:** Duplicate IDs override wrong app.
  - **Mitigation:** Duplicate detection with explicit overwrite confirmation (phase 2); for now reject duplicates.
- **Risk:** Main update loop becomes harder to maintain.
  - **Mitigation:** Keep parsing/validation logic in `internal/customapps` package, keep `main.go` glue-only.

## Out of Scope (This Iteration)

- Edit/delete existing custom entries.
- Interactive filesystem picker.
- Per-custom-entry sync mode defaults.
- Import/export custom entries from remote sources.

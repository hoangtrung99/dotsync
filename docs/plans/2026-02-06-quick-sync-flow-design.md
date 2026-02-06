# Quick Sync Flow Design

> Tối ưu flow đồng bộ dotfiles giữa nhiều máy với conflict resolution qua IDE

## Tổng quan

### Vấn đề hiện tại

1. **Phát hiện conflict chậm** - Không biết có conflict cho đến khi đã quá muộn
2. **Merge tool khó dùng** - TUI merge view không đủ mạnh
3. **Mất context** - Khó quyết định giữ version nào
4. **Workflow rời rạc** - Quá nhiều bước thủ công (scan → select → diff → merge → commit)

### Giải pháp

- **Quick Sync Mode**: Một phím `Q` để fetch → detect → auto-resolve hoặc mở IDE
- **IDE Integration**: Tự động mở VS Code/Cursor/Zed khi có conflict
- **Smart Suggestions**: Gợi ý action phù hợp dựa trên trạng thái

## Use Case

- 2 máy (work + personal) đều có thể push changes
- Switch giữa máy tùy theo tác vụ
- Cần on-demand conflict detection (user chủ động check)

---

## Quick Sync Flow

### Trigger

Nhấn phím `Q` từ màn hình chính.

### Flow Diagram

```
Q pressed
    │
    ▼
┌──────────────┐
│ Git fetch    │ ← Lấy updates từ remote
└──────────────┘
    │
    ▼
┌──────────────┐
│ Detect state │ ← So sánh local vs remote vs dotfiles
└──────────────┘
    │
    ├─── No changes ──→ "✓ Everything synced"
    │
    ├─── Local only ──→ Auto-push + commit
    │
    ├─── Remote only ─→ Auto-pull
    │
    └─── Conflicts ───→ Open IDE with conflict files
```

### State Detection

| State | Condition | Action |
|-------|-----------|--------|
| Synced | local == remote == dotfiles | Hiển thị `✓ All synced` |
| Local modified | local != dotfiles, remote == dotfiles | Auto-push + commit |
| Remote updated | local == dotfiles, remote != dotfiles | Auto-pull |
| Conflict | local != remote, cả hai != base | Mở IDE |

---

## IDE Integration

### Supported Editors

1. **VS Code** - `code --wait --merge`
2. **Cursor** - `cursor --wait --merge`
3. **Zed** - `zed` (workspace mode)

### Auto-detection Priority

```json
{
  "editor": "auto",
  "editor_priority": ["cursor", "code", "zed"]
}
```

### Merge Flow

```
Phát hiện conflict
    │
    ▼
┌──────────────────┐
│ Tạo temp folder  │
│ ~/.dotsync/merge │
└──────────────────┘
    │
    ▼
┌──────────────────┐
│ Copy 3 versions: │
│  • LOCAL.ext     │ ← File từ máy hiện tại
│  • REMOTE.ext    │ ← File từ dotfiles repo
│  • MERGED.ext    │ ← File kết quả (user edit)
└──────────────────┘
    │
    ▼
┌──────────────────┐
│ Mở IDE với args  │
│ (tự động)        │
└──────────────────┘
    │
    ▼
┌──────────────────┐
│ Watch MERGED.ext │ ← Đợi user save
│ Apply changes    │
│ Cleanup temp     │
└──────────────────┘
```

### Editor Commands

```bash
# VS Code - 3-way merge editor
code --wait --merge LOCAL REMOTE BASE MERGED

# Cursor - tương tự VS Code
cursor --wait --merge LOCAL REMOTE BASE MERGED

# Zed - mở workspace với 3 files
zed LOCAL REMOTE MERGED
```

---

## Smart Suggestions

### UI Location

Suggestion bar hiển thị ở đầu màn hình chính, phía trên app list.

### Mockup

```
┌─────────────────────────────────────────────────────────┐
│ 🔄 Dotsync v1.0                        ~/dotfiles [main]│
├─────────────────────────────────────────────────────────┤
│                                                         │
│  💡 SUGGESTED ACTION:                                   │
│  ┌─────────────────────────────────────────────────┐    │
│  │ ↑ 3 files modified locally (zsh, nvim, git)    │    │
│  │                                                 │    │
│  │   [P] Push now    [Q] Quick sync    [D] Details │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
├─────────────────────────────────────────────────────────┤
│ Apps (42)                    │ Files                    │
```

### Suggestion Types

| State | Message | Actions |
|-------|---------|---------|
| Local modified | `↑ N files modified locally` | `[P] Push` `[Q] Quick sync` |
| Remote updated | `↓ N updates available` | `[L] Pull` `[Q] Quick sync` |
| Conflicts | `⚡ N conflicts detected` | `[Q] Quick sync to resolve` |
| All synced | `✓ Everything synced` | (ẩn bar) |
| First run | `👋 Welcome! Select apps` | `[A] Select all` |

---

## Key Bindings

### New Keys

| Phím | Action | Mô tả |
|------|--------|-------|
| `Q` | Quick Sync | Fetch → Detect → Auto-resolve hoặc mở IDE |
| `E` | Open in Editor | Mở file/conflict hiện tại trong IDE |
| `C` | Check conflicts | Scan conflicts on-demand |
| `Shift+P` | Push + Commit | Push và tự tạo commit message |

### Existing Keys (unchanged)

- `P` - Push selected
- `L` - Pull selected
- `D` - View diff
- `G` - Git panel
- `/` - Search
- `?` - Help

---

## Architecture

### New Modules

```
dotsync/
├── internal/
│   ├── quicksync/           # NEW
│   │   ├── quicksync.go     # Main orchestrator
│   │   ├── detector.go      # Conflict detection
│   │   └── resolver.go      # Auto-resolve logic
│   │
│   ├── editor/              # NEW
│   │   ├── editor.go        # Interface + auto-detect
│   │   ├── vscode.go        # VS Code implementation
│   │   ├── cursor.go        # Cursor implementation
│   │   ├── zed.go           # Zed implementation
│   │   └── watcher.go       # File watcher for merge result
│   │
│   ├── suggestions/         # NEW
│   │   └── suggestions.go   # Analyze state → suggest action
│   │
│   └── ... (existing)
```

### Interfaces

```go
// Editor interface
type Editor interface {
    Name() string
    IsInstalled() bool
    OpenMerge(local, remote, merged string) error
    OpenDiff(file1, file2 string) error
    Wait() error
}

// QuickSync orchestrator
type QuickSync struct {
    config   *config.Config
    git      *git.Repo
    editor   Editor
    detector *ConflictDetector
}

func (q *QuickSync) Run() (*Result, error)

// Result types
type Result struct {
    Action      ActionType  // Synced, Pushed, Pulled, Merged
    FilesCount  int
    Conflicts   []ConflictFile
    Error       error
}

// Suggestion
type Suggestion struct {
    Type    SuggestionType
    Message string
    Actions []Action
    Files   []string
}
```

### Config Changes

```go
type Config struct {
    // ... existing fields

    // New fields
    Editor         string   `json:"editor"`          // auto, code, cursor, zed
    EditorPriority []string `json:"editor_priority"` // fallback order
    AutoCommit     bool     `json:"auto_commit"`     // auto commit on push
    CommitTemplate string   `json:"commit_template"` // default: "sync: update {files}"
}
```

---

## Workflow Examples

### Scenario 1: Simple Push (no conflicts)

```
MÁY A (Work):
1. User sửa .zshrc
2. Mở dotsync
3. Thấy: "↑ 1 file modified"
4. Nhấn Q
5. Dotsync: fetch → no remote changes → push → commit
6. Done (5 giây)
```

### Scenario 2: Simple Pull (no conflicts)

```
MÁY B (Personal):
1. Mở dotsync
2. Thấy: "↓ 1 update available"
3. Nhấn Q
4. Dotsync: fetch → pull → apply
5. Done (3 giây)
```

### Scenario 3: Conflict Resolution

```
MÁY B (Personal) - quên pull trước khi sửa:
1. Mở dotsync
2. Thấy: "⚡ 1 conflict detected"
3. Nhấn Q
4. VS Code tự mở với 3-way merge
5. User resolve trong VS Code, save
6. Dotsync detect save → apply → commit
7. Done
```

---

## Performance Goals

| Metric | Target |
|--------|--------|
| Quick Sync (no conflict) | < 3 giây |
| Conflict detection | < 1 giây |
| IDE launch | < 2 giây |
| Total workflow (simple sync) | < 5 giây |

---

## Migration

### Backward Compatibility

- Tất cả phím tắt cũ vẫn hoạt động
- Quick Sync là tính năng bổ sung, không thay thế flow cũ
- User có thể tiếp tục dùng manual flow nếu muốn

### New Defaults

- `editor: "auto"` - tự detect IDE
- `auto_commit: true` - tự commit khi push
- Suggestion bar hiển thị mặc định

---

## Implementation Plan

### Phase 1: Core Quick Sync
1. Implement `internal/quicksync` module
2. Add `Q` key binding
3. Implement state detection logic
4. Auto push/pull for simple cases

### Phase 2: IDE Integration
1. Implement `internal/editor` module
2. VS Code support
3. Cursor support
4. Zed support
5. File watcher for merge completion

### Phase 3: Smart Suggestions
1. Implement `internal/suggestions` module
2. Add suggestion bar UI component
3. Integrate with main screen

### Phase 4: Polish
1. Config options
2. Error handling
3. Testing
4. Documentation

---

## Open Questions

1. **Auto-commit message format**: Dùng template gì?
   - Đề xuất: `sync: update {app} ({n} files)`

2. **Multiple conflicts**: Mở từng file hay tất cả cùng lúc trong IDE?
   - Đề xuất: Mở tất cả trong cùng workspace

3. **Fallback khi không có IDE**: Dùng TUI merge hay báo lỗi?
   - Đề xuất: Fallback về TUI merge hiện tại

# Backup Mode Design

> Hỗ trợ lưu trữ config riêng biệt theo máy, song song với chế độ Sync

## Tổng quan

### Vấn đề

Một số config cần giống nhau trên mọi máy (sync), nhưng một số khác cần giữ riêng biệt theo từng máy (backup). Ví dụ:
- `.zshrc` - sync (giống nhau)
- `.zshrc.local` - backup (PATH, aliases riêng mỗi máy)

### Giải pháp

Thêm 2 chế độ hoạt động cho mỗi app/file:
- **Sync**: Giống nhau trên mọi máy, xử lý thủ công
- **Backup**: Riêng biệt theo máy, tự động push

---

## Concept: Sync vs Backup Mode

| Mode | Mô tả | Storage | Xử lý |
|------|-------|---------|-------|
| **Sync** | Giống nhau trên mọi máy | `dotfiles/{app}/{file}` | Thủ công (P/L) |
| **Backup** | Riêng biệt theo máy | `dotfiles/{app}/{machine}/{file}` | Tự động |

### Storage Structure

```
dotfiles/
├── .dotsync/                 # Metadata
│   └── machines.json         # List of known machines
│
├── zsh/                      # App: zsh
│   ├── .zshrc                # MODE: SYNC (shared)
│   ├── machine-a/
│   │   └── .zshrc.local      # MODE: BACKUP (machine-a only)
│   └── machine-b/
│       └── .zshrc.local      # MODE: BACKUP (machine-b only)
│
├── nvim/                     # App: nvim (all BACKUP)
│   ├── machine-a/
│   │   └── init.lua
│   └── machine-b/
│       └── init.lua
│
└── git/                      # App: git (all SYNC)
    ├── .gitconfig
    └── .gitignore_global
```

---

## Mode Configuration

### Config File

`~/.config/dotsync/modes.json`:

```json
{
  "machine_name": "machine-a",
  "default_mode": "backup",

  "apps": {
    "git": "sync",
    "zsh": "sync",
    "nvim": "backup"
  },

  "files": {
    "zsh/.zshrc": "sync",
    "zsh/.zshrc.local": "backup",
    "git/.gitconfig": "sync",
    "git/.gitconfig.local": "backup"
  }
}
```

### Priority Rules

```
file override > app setting > default_mode
```

### Default Mode

**Backup** là default - an toàn hơn, không ghi đè nhầm config của máy khác.

---

## UI Changes

### Mode Indicators

```
┌─────────────────────────────────────────────────────────┐
│ Apps (42)                    │ Files                    │
│ ─────────────────────────────│──────────────────────────│
│ > ● zsh          [S]         │   .zshrc         [S]     │
│   ● neovim       [B]         │   .zshrc.local   [B]     │
│   ✓ git          [S]         │   .zprofile      [S]     │
│   ○ tmux         [B]         │                          │
├─────────────────────────────────────────────────────────┤
│ M toggle  Shift+S all sync  Shift+B all backup  ? help │
└─────────────────────────────────────────────────────────┘
```

### Key Bindings

| Phím | Action | Mô tả |
|------|--------|-------|
| `M` | Toggle mode | Đổi mode của item đang chọn (Sync ↔ Backup) |
| `Shift+S` | Set all Sync | Đặt tất cả items đang hiển thị = Sync |
| `Shift+B` | Set all Backup | Đặt tất cả items đang hiển thị = Backup |
| `R` | Restore from... | Mở dialog restore từ máy khác |

---

## Quick Sync với cả 2 Modes

### Flow

```
Q pressed (Quick Sync)
    │
    ▼
┌────────────────────────────────────┐
│ Git fetch + Phân loại files       │
└────────────────────────────────────┘
    │
    ├─── BACKUP FILES ─────────────────────────────┐
    │    │                                         │
    │    └── Auto push → dotfiles/app/{machine}/   │
    │        (tự động, không cần confirm)          │
    │                                              │
    └─── SYNC FILES ───────────────────────────────┐
         │                                         │
         └── Chỉ báo cáo trạng thái, KHÔNG tự xử lý│
             User phải dùng P/L để push/pull       │
```

### Output

```
✓ Quick Sync completed:
  Backed up: 5 files → machine-a/

⚠ Sync files need manual action:
  ↑ 2 modified (zsh, git) → press P to push
  ↓ 1 outdated (tmux) → press L to pull
```

---

## Cross-machine Restore

### Trigger

Nhấn `R` từ màn hình chính.

### UI Dialog

```
┌─────────────────────────────────────────────────────────┐
│  📥 Restore from another machine                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Select source machine:                                 │
│                                                         │
│  > [1] machine-a     (last sync: 2 hours ago)          │
│    [2] machine-b     (last sync: 1 day ago)            │
│    [3] old-laptop    (last sync: 30 days ago)          │
│                                                         │
├─────────────────────────────────────────────────────────┤
│  Files to restore:                                      │
│                                                         │
│  [x] nvim/init.lua                                     │
│  [x] zsh/.zshrc.local                                  │
│  [ ] tmux/.tmux.conf                                   │
│                                                         │
├─────────────────────────────────────────────────────────┤
│  ↑↓ navigate  Space select  Enter restore  Esc cancel  │
└─────────────────────────────────────────────────────────┘
```

### Behavior

1. Nhấn `R` → Hiện list machines có backup
2. Chọn source machine
3. Chọn files cần restore
4. Enter → Backup file hiện tại trước
5. Copy files từ `dotfiles/app/{source-machine}/` → local

---

## Architecture

### New Modules

```
dotsync/
├── internal/
│   ├── modes/                # NEW: Mode management
│   │   ├── modes.go          # Load/save modes config
│   │   └── resolver.go       # Resolve mode cho app/file
│   │
│   ├── backup/               # NEW: Backup operations
│   │   ├── backup.go         # Push to machine folder
│   │   └── restore.go        # Restore from other machine
│   │
│   ├── quicksync/            # UPDATE
│   │   └── quicksync.go      # Thêm logic phân loại mode
│   │
│   └── ui/components/        # UPDATE
│       ├── applist.go        # Thêm [S]/[B] indicator
│       ├── filelist.go       # Thêm [S]/[B] indicator
│       └── restoredialog.go  # NEW: Restore dialog
```

### Interfaces

```go
// modes/modes.go
type ModesConfig struct {
    MachineName string            `json:"machine_name"`
    DefaultMode Mode              `json:"default_mode"`
    Apps        map[string]Mode   `json:"apps"`
    Files       map[string]Mode   `json:"files"`
}

type Mode string

const (
    ModeSync   Mode = "sync"
    ModeBackup Mode = "backup"
)

// GetMode returns mode for a specific file
// Priority: file override > app setting > default
func (m *ModesConfig) GetMode(appID, filePath string) Mode

// backup/backup.go
type BackupManager struct {
    config      *config.Config
    modesConfig *modes.ModesConfig
    git         *git.Repo
}

func (b *BackupManager) Backup(apps []*models.App) (*BackupResult, error)
func (b *BackupManager) ListMachines() ([]Machine, error)
func (b *BackupManager) Restore(sourceMachine string, files []string) error

// Machine info
type Machine struct {
    Name     string
    LastSync time.Time
    Files    []string
}
```

### Machines Metadata

`dotfiles/.dotsync/machines.json`:

```json
{
  "machines": [
    {
      "name": "machine-a",
      "last_sync": "2026-02-06T10:30:00Z"
    },
    {
      "name": "machine-b",
      "last_sync": "2026-02-05T15:45:00Z"
    }
  ]
}
```

---

## Implementation Plan

### Phase 1: Mode System
1. Implement `internal/modes` module
2. Add modes.json config file
3. Add `M`, `Shift+S`, `Shift+B` key bindings
4. Update UI with [S]/[B] indicators

### Phase 2: Backup Operations
1. Implement `internal/backup` module
2. Update Quick Sync to handle backup files
3. Auto-push backup files to machine folder

### Phase 3: Restore Feature
1. Implement restore dialog UI
2. Add `R` key binding
3. List machines from dotfiles repo
4. Restore files with backup of current version

### Phase 4: Integration
1. Update Quick Sync flow
2. Update suggestions for mixed mode
3. Testing & documentation

---

## Comparison with Quick Sync Design

| Feature | Quick Sync | Backup Mode |
|---------|------------|-------------|
| Storage | Single location | Per-machine folders |
| Conflict | Can occur | Never (separate folders) |
| Auto-sync | Yes (for simple cases) | Yes (always) |
| Manual action | For conflicts | For Sync mode files |
| Cross-machine | Pull same config | Restore from specific machine |

Hai tính năng hoạt động **song song**:
- Quick Sync (`Q`) xử lý cả Sync và Backup files
- Backup files được auto-push
- Sync files được báo cáo để user xử lý thủ công

# kkonf - kubectl Config Manager

[![Latest Release](https://img.shields.io/github/release/positronico/kkonf.svg)](https://github.com/positronico/kkonf/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/positronico/kkonf/total.svg)](https://github.com/positronico/kkonf/releases)

`kkonf` is a full-screen terminal UI for managing kubectl configuration files, with
scriptable subcommands for everyday context switching, and a focus on consolidating
duplicate users across GKE/EKS/AKS clusters.

## 📥 Quick Download

**→ [Download pre-built binaries from GitHub Releases](https://github.com/positronico/kkonf/releases/latest) ←**

Choose the appropriate binary for your platform (Linux, macOS, Windows) from the latest release page.

## Features

- **Full-screen TUI**: persistent header (file, current context, modified state),
  filterable tables, all-fields-at-once forms with masked secrets, toast
  notifications — no more "press Enter to continue"
- **Number-key navigation**: `1`-`5` jump between sections; in pickers, `1`-`9`
  select directly and `0`/`Esc` cancels
- **Scriptable subcommands**: `kkonf ctx`, `kkonf ns`, `kkonf rename`,
  `kkonf consolidate --dry-run`, `kkonf export`, `kkonf backup` — kubectx-style
  one-liners for shells and CI
- **Lossless round-trip**: unknown and future kubeconfig fields (impersonation,
  extensions at every level, vendor extras) survive load → save untouched
- **User consolidation**: detect users with identical auth settings and merge
  them, rewriting every context reference
- **Safe writes**: per-save timestamped backups, atomic temp+rename writes
  through symlinks, file locking against concurrent writers, and detection of
  external changes (e.g. kubectl modifying the file while kkonf is open)
- **Import/Export**: merge another kubeconfig with conflict handling
  (skip / replace / rename), export contexts with their dependencies
- **Validation**: broken references, duplicates, orphans, missing auth

## Installation

### Option 1: Download Pre-built Binaries (Recommended)

**→ [Go to Releases Page](https://github.com/positronico/kkonf/releases/latest) ←**

1. Download the binary for your platform (Linux/macOS/Windows, amd64/arm64)
2. Extract and run:
   ```bash
   tar -xzf kkonf-v*.tar.gz
   ./kkonf
   ```

### Option 2: Install with Go
```bash
go install github.com/positronico/kkonf@latest
```

### Option 3: Build from Source
**Prerequisites:** Go 1.24 or higher

```bash
git clone https://github.com/positronico/kkonf.git
cd kkonf
make build  # or: go build -o kkonf
```

## Usage

### Interactive TUI
```bash
kkonf                     # default kubeconfig (~/.kube/config)
kkonf -f /path/to/config  # a specific file
```

```
┌ kkonf v2.0.0 │ ~/.kube/config │ ctx: prod-east │ saved ──────────┐
│ [1 Clusters] [2 Users] [3 Contexts] [4 Tools] [5 Settings]       │
│                                                                  │
│    Name          Cluster        User        Namespace            │
│  ● prod-east     prod-east      exec-user   payments             │
│    dev           dev-local      token-user  default              │
│                                                                  │
│ ✓ Current context: prod-east                                     │
└ enter/s switch  n namespace  a add  e edit  r rename  d delete ──┘
```

The TUI opens on the **Contexts** screen, so switching context is: arrow (or
`/` to filter) + `Enter`. Changes stay in memory until you save.

| Key | Action |
|-----|--------|
| `1`-`5` | Jump to Clusters / Users / Contexts / Tools / Settings |
| `Enter`/`s` | Switch current context (Contexts screen) |
| `a` `e` `r` `d` | Add / Edit / Rename / Delete the selected entry |
| `v` | View details (Clusters, Users) |
| `n` | Set namespace (Contexts) |
| `c` | Consolidate duplicate users (Users) |
| `/` | Filter the table |
| `Ctrl+S` | Save (validates first, warns on external changes) |
| `q`/`Esc` | Quit (prompts to save or discard changes) |
| `1`-`9`, `0` | In pickers: select option N directly, `0` cancels |

### Subcommands (non-interactive)
```bash
kkonf ctx                        # list contexts (current marked with *)
kkonf ctx staging                # switch context
kkonf ns payments                # set namespace of the current context
kkonf rename cluster old new     # rename + update all references
kkonf consolidate --dry-run      # preview duplicate-user merging
kkonf consolidate                # merge duplicate users and save
kkonf export -o team.yaml prod   # export a context with its cluster + user
kkonf backup list                # list timestamped backups
kkonf backup restore             # roll back to the newest backup
```

All subcommands honor `-f/--file` and generate shell completion via
`kkonf completion bash|zsh|fish`.

## User Consolidation

Managing many GKE/EKS/AKS clusters usually leaves you with one identical
exec-auth user per cluster. kkonf groups users whose entire definition is
identical and merges each group into one user, updating every context:

```bash
$ kkonf consolidate --dry-run
Would consolidate [gke_p1_c1 gke_p1_c2 gke_p2_c1] into "gke-user" (exec auth)
```

In the TUI: Users screen (`2`), then `c`.

## Safety Model

- Every save first copies the current file to `config.bak.YYYYMMDD-HHMMSS`
  (each save gets its own backup file)
- Writes are atomic (temp file + rename), preserve the original file's
  permissions, and follow symlinks instead of replacing them
- A lock file serializes concurrent writers; stale locks from crashed
  processes are broken automatically
- If another tool changed the file since kkonf loaded it, saving asks before
  overwriting
- `kkonf backup restore` backs up the current state before restoring, so a
  restore is itself undoable
- Backup cleanup keeps everything newer than the retention window **and** the
  newest N backups regardless of age (configurable in Settings)

## Known Limitations

- YAML comments and anchors/aliases are not preserved on save (kubectl drops
  them too); all field *content* — including unknown fields — is preserved
- Users authenticated via the legacy `auth-provider` field are shown and can
  be renamed/deleted, but not edited (editing would convert them)

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

MIT License - See LICENSE file for details

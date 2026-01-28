# Rift

**Rift** is a lightweight Go-based CLI tool designed to instantly "teleport" your project files from your current working directory to a specific destination.

### Features

* **Zero Config**: Just point and shoot.
* **Smart Sync**: Syncs content into a folder with the same name as your project.
* **Gitignore Support**: Automatically respects `.gitignore` patterns (and always excludes `.git`).
* **True Sync**: Removes orphaned files from destination that no longer exist in source.
* **Incremental**: Skips unchanged files (same size and modification time).

---

### Installation

```bash
go install github.com/byteorem/rift@latest
```

### Usage

```
rift --to <destination> [--exclude <pattern>]...
```

**Flags:**
- `--to` — Destination path (required)
- `--name` — Name for destination folder (defaults to current directory name)
- `--exclude` — Additional patterns to exclude (repeatable)
- `-h, --help` — Show help

**Examples:**

```bash
# Basic sync
rift --to /backup

# Custom destination folder name
rift --to /games/addons --name MyAddon

# Sync with additional exclusions
rift --to ~/projects-backup --exclude "*.log" --exclude "tmp/"
```

If you are in `~/projects/my-addon` and run `rift --to "/games/addons"`, it will sync everything to `/games/addons/my-addon/`.


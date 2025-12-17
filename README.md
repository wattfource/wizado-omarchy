## wizado

Scaffold for an Arch Linux (and future AUR) “game launcher” installer project.

### What’s here

- **`scripts/setup.sh`**: install/setup (you will paste your real setup logic here)
- **`scripts/update.sh`**: update/refresh anything installed by setup
- **`scripts/remove.sh`**: remove/undo anything installed by setup
- **`scripts/lib/common.sh`**: shared helpers (logging, root checks, state file)
- **`.state/`**: local state (tracked in gitignore)

### Quick start

Run from the repo root:

```bash
bin/wizado setup
```

### Notes

- The scripts are intentionally **minimal** and **safe by default** (explicit `--yes` for non-interactive).
- `setup.sh` writes an “installed items” state file. `remove.sh` uses it to undo work.
## Scripts

### Files

- **`setup.sh`**: paste your real setup logic here. Use `record_installed_item` for anything you want `remove.sh` to undo.
- **`update.sh`**: update system and/or your installed artifacts.
- **`remove.sh`**: removes items in `.state/installed_items.txt`.
- **`lib/common.sh`**: shared helpers.

### State tracking

`setup.sh` overwrites `.state/installed_items.txt` at the beginning of a run.

Supported item formats:

- `pacman:<pkgname>`
- `file:</absolute/or/relative/path>`
- `setcap:</path/to/binary>`
- `hyprsource:</path/to/hyprland.conf>`


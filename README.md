# Wizado

Steam gaming mode for Hyprland on Arch Linux (Omarchy).

Launches Steam in a gamescope session with simple keybindings.

## Features

- Launch Steam Big Picture in gamescope with `Super + Shift + S`
- Exit gaming mode with `Super + Shift + R`
- Auto-detects display resolution and refresh rate
- Suspends hypridle during gaming to prevent screen blanking
- Uses gamemode for system optimizations (if installed)

## Requirements

- Arch Linux with Omarchy
- Hyprland
- Steam
- gamescope

## Installation

### From Source

```bash
git clone https://github.com/REPLACE_ME/wizado.git
cd wizado
./scripts/setup.sh
```

### As Arch Package

```bash
makepkg -si
wizado setup
```

## Usage

After installation:

| Keybinding | Action |
|------------|--------|
| `Super + Shift + S` | Launch Steam in gamescope |
| `Super + Shift + R` | Exit gaming mode |

## Uninstall

```bash
wizado remove
```

Or if installed from source:

```bash
./scripts/remove.sh
```

## What Gets Installed

- Launcher scripts in `~/.local/share/steam-launcher/`
- Keybindings added to your Hyprland config
- Steam and gaming dependencies (via pacman)

## Deferred Features

The following features are planned but not yet implemented:

- Performance mode (dedicated TTY, CPU/GPU optimization)
- TUI configuration menu
- Waybar integration
- Custom resolution/upscaling options

## License

MIT

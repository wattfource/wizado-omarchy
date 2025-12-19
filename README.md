# Wizado

Steam gaming mode for Hyprland on Arch Linux (Omarchy).

Launches Steam in a gamescope session with simple keybindings.

## Features

- Launch Steam (Deck UI) in gamescope with `Super + Shift + S`
- Exit gaming mode with `Super + Shift + R`
- Auto-detects display resolution and refresh rate
- Suspends hypridle during gaming to prevent screen blanking
- MangoHud integration (if installed)
- Gamemode support for individual games

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

### Using Gamemode with Games

Gamemode optimizes your system while gaming (CPU governor, I/O priority, etc.). 
To enable it for a specific game:

1. Right-click the game in Steam → Properties
2. In "Launch Options", add:
   ```
   gamemoderun %command%
   ```

### MangoHud Performance Overlay

If MangoHud is installed, it's automatically available via gamescope's `--mangoapp` integration.
Press `Super + F1` in-game to toggle the overlay (or configure via MangoHud settings).

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
- Steam and gaming dependencies (via pacman):
  - `gamescope` - Micro-compositor for games
  - `gamemode` + `lib32-gamemode` - System optimization daemon
  - `mangohud` + `lib32-mangohud` - Performance overlay

## Technical Details

### How it Works

1. **Gamescope** runs as a nested Wayland compositor inside Hyprland
2. **Steam** launches in Deck UI mode (`-steamdeck`) for optimal gamescope integration
3. **Games** run inside gamescope with proper frame pacing and VSync handling
4. **Gamemode** can be enabled per-game via Steam launch options

### Why Not `gamemoderun gamescope`?

Gamemode should optimize the **game process**, not the compositor. Running 
`gamemoderun gamescope` would apply optimizations to gamescope itself, which 
doesn't help game performance. Instead, use `gamemoderun %command%` in each 
game's launch options.

## Deferred Features

The following features are planned but not yet implemented:

- Performance mode (dedicated TTY, CPU/GPU optimization)
- TUI configuration menu
- Waybar integration
- Custom resolution/upscaling options

## License

MIT

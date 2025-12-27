# Wizado

Steam gaming launcher for Hyprland on Arch Linux (Omarchy).

Launches Steam in a fullscreen gamescope session on a dedicated workspace.

## Features

- Single compiled binary (no bash scripts to bypass)
- License-protected with HMAC-signed local validation
- TUI configuration with Bubbletea
- FSR upscaling support
- Frame rate limiting
- VRR/Adaptive Sync
- NVIDIA GPU auto-detection
- Waybar integration

## Requirements

- Arch Linux with Hyprland
- Steam
- gamescope

## Installation

### AUR (Recommended)

```bash
yay -S wizado
wizado setup
```

### From Source

```bash
git clone https://github.com/wattfource/wizado-omarchy.git
cd wizado-omarchy
make build
sudo make install
wizado setup
```

### One-Liner

```bash
curl -fsSL https://wizado.app/install.sh | bash
```

## License

Wizado requires a valid license to run.

- **Price:** $10 for 5 machines
- **Get a license:** [wizado.app](https://wizado.app)

On first launch, you'll be prompted to enter your email and license key.

## Usage

| Command | Action |
|---------|--------|
| `wizado` | Launch Steam (with license check) |
| `wizado config` | Configure settings & license via TUI |
| `wizado setup` | Install dependencies and configure system |
| `wizado status` | Output JSON for waybar |
| `wizado activate EMAIL KEY` | Activate license (non-interactive) |
| `wizado remove` | Remove configuration and keybindings |

### Keybindings (after setup)

| Keys | Action |
|------|--------|
| `Super + Shift + S` | Launch Steam |
| `Super + Shift + Q` | Force-quit Steam + gamescope |

### Waybar Module

After `wizado setup`, an icon () appears in your waybar:

- **Left-click:** Launch Steam (or enter license if not activated)
- **Right-click:** Open settings TUI

### Configuration

Settings stored in `~/.config/wizado/config`:

```bash
WIZADO_RESOLUTION=auto    # auto, 1920x1080, 2560x1440, etc.
WIZADO_FSR=off            # off, ultra, quality, balanced, performance
WIZADO_FRAMELIMIT=0       # 0 = unlimited, or FPS cap (60, 120, etc.)
WIZADO_VRR=off            # on/off - Variable Refresh Rate
WIZADO_MANGOHUD=off       # on/off - Performance overlay
WIZADO_STEAM_UI=tenfoot   # tenfoot (Big Picture) or gamepadui (Steam Deck UI)
WIZADO_WORKSPACE=10       # Preferred workspace (1-10)
```

## How It Works

When you run `wizado` or press `Super + Shift + S`:

1. Validates your license (HMAC-signed, machine-bound)
2. Finds an empty workspace (or uses workspace 10)
3. Switches to that workspace
4. Launches gamescope + Steam in fullscreen
5. When you exit Steam, returns to your original workspace

## Building

```bash
# Build
make build

# Install to /usr/bin
sudo make install

# Clean
make clean
```

## Uninstall

```bash
wizado remove
sudo pacman -R wizado  # if installed via AUR
```

## Technical Notes

### Gamescope Flags Used

```bash
-W/-H                      # Output resolution (your monitor)
-w/-h                      # Render resolution (for FSR)
-f                         # Fullscreen
-e                         # Steam integration
--force-windows-fullscreen # Better game compatibility
--disable-color-management # CRITICAL for NVIDIA
--prefer-vk-device XXXX    # Force specific GPU
```

### NVIDIA Environment Variables

```bash
VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/nvidia_icd.json
__GLX_VENDOR_LIBRARY_NAME=nvidia
__GL_SHADER_DISK_CACHE=1
WLR_NO_HARDWARE_CURSORS=1
```

## License

MIT

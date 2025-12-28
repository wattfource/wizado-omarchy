# Wizado

Steam gaming launcher for Hyprland on Arch Linux (Omarchy).

Launches Steam in a fullscreen gamescope session on a dedicated workspace, optimized for desktop gaming with keyboard, mouse, and optional controller support.

## Features

- Single compiled binary (no bash scripts to bypass)
- License-protected with HMAC-signed local validation
- TUI configuration with Bubbletea
- FSR upscaling support
- Frame rate limiting
- VRR/Adaptive Sync
- **GameMode integration** for maximum performance
- NVIDIA GPU auto-detection and optimization
- AMD/Intel GPU support
- **Comprehensive input device detection** (keyboard, mouse, controller)
- **Structured logging** with rotation
- **Anonymous telemetry** (local storage only, phase 2 will add opt-in reporting)
- **System information collection** for diagnostics
- Waybar integration
- Internet connectivity checks

## Requirements

- Arch Linux with Hyprland
- Steam
- gamescope

## Optional (Recommended)

- **gamemode** - CPU/GPU performance optimizations
- **mangohud** - Performance overlay (FPS, temps, etc.)

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
| `wizado info` | Display system information and diagnostics |
| `wizado info --json` | Output system info as JSON |
| `wizado logs` | View application logs |
| `wizado logs -f` | Follow log output in real-time |
| `wizado logs --session` | View latest gaming session log |
| `wizado logs --clear` | Clear all logs |
| `wizado status` | Output JSON for waybar |
| `wizado activate EMAIL KEY` | Activate license (non-interactive) |
| `wizado remove` | Remove configuration and keybindings |
| `wizado-menu` | Open TUI menu |
| `wizado-menu-float` | Open TUI in floating terminal |

### Keybindings (after setup)

| Keys | Action |
|------|--------|
| `Super + Shift + S` | Launch Steam |
| `Super + Shift + Q` | Force-quit Steam + gamescope |

### Waybar Module

After `wizado setup`, an icon () appears in your waybar:

- **Left-click:** Open Wizado menu
- **Right-click:** Open Wizado menu

### Hyprland Window Rules

Add these to `~/.config/hypr/hyprland.conf` for the floating menu:

```conf
windowrulev2 = float, class:^(wizado-menu)$
windowrulev2 = center, class:^(wizado-menu)$
windowrulev2 = size 500 400, class:^(wizado-menu)$
```

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
2. Collects system information for optimal configuration
3. Activates GameMode (if installed) for CPU/GPU optimizations
4. Finds an empty workspace (or uses workspace 10)
5. Stops hypridle to prevent screen blanking
6. Switches to that workspace
7. Launches gamescope + Steam in fullscreen
8. Logs session data for diagnostics
9. When you exit Steam, returns to your original workspace
10. Deactivates GameMode and restarts hypridle

## System Detection

Wizado automatically detects:

### Hardware
- **GPU**: NVIDIA, AMD, Intel (with driver version for NVIDIA)
- **CPU**: Model, cores, threads
- **RAM**: Total and available memory
- **Display**: Resolution, refresh rate, scale factor

### Input Devices
- Keyboards
- Mice/Trackpads
- Game Controllers (Xbox, PlayStation, Steam Controller, etc.)

### Network
- Internet connectivity
- Connection type (Ethernet/WiFi)
- WiFi SSID (when applicable)

### Software
- OS name and version
- Kernel version
- Hyprland version
- Steam, Gamescope, GameMode, MangoHUD versions

Run `wizado info` to see all detected information.

## Performance Features

### GameMode

When GameMode is installed, Wizado automatically:
- Requests GameMode activation before launching Steam
- Enables CPU governor optimizations
- Applies GPU performance tweaks
- Deactivates GameMode when you exit Steam

### NVIDIA Optimizations

For NVIDIA GPUs, Wizado sets:
- Vulkan ICD for NVIDIA
- GLX vendor library
- Shader disk cache
- G-Sync/VRR support
- Threaded optimizations

### Gamescope Flags

```bash
-W/-H                      # Output resolution (your monitor)
-w/-h                      # Render resolution (for FSR)
-f                         # Fullscreen
-e                         # Steam integration
--force-windows-fullscreen # Better game compatibility
--disable-color-management # CRITICAL for NVIDIA
--prefer-vk-device XXXX    # Force specific GPU
--adaptive-sync            # VRR support
--framerate-limit          # FPS limiting
```

## Logging

Wizado maintains several log files:

- `~/.cache/wizado/wizado.log` - Main application log (with rotation)
- `~/.cache/wizado/sessions/session_<id>.log` - Per-session logs
- `~/.cache/wizado/latest-session.log` - Symlink to most recent session

View logs with:
```bash
wizado logs           # View main log
wizado logs -f        # Follow log in real-time
wizado logs --session # View latest session log
```

## Telemetry

Wizado collects anonymous usage data locally for diagnostics:

- **Event types**: Launch, exit, errors, configuration changes
- **System snapshots**: Hardware info, dependency versions
- **Storage**: `~/.local/share/wizado/telemetry/`

This data is stored locally and NOT sent anywhere (Phase 2 will add opt-in remote reporting).

To clear telemetry data:
```bash
rm -rf ~/.local/share/wizado/telemetry/
```

## Building

```bash
# Build
make build

# Install to /usr/bin
sudo make install

# Uninstall
sudo make uninstall

# Clean build artifacts
make clean

# Run tests
make test
```

## Uninstall

```bash
wizado remove
sudo pacman -R wizado  # if installed via AUR
# or
sudo make uninstall    # if installed from source
```

## Technical Notes

### Environment Variables Set

```bash
# Performance
SDL_VIDEO_MINIMIZE_ON_FOCUS_LOSS=0
STEAM_RUNTIME_PREFER_HOST_LIBRARIES=0
STEAM_FORCE_DESKTOPUI_SCALING=1

# Gamescope integration
STEAM_GAMESCOPE_NIS_SUPPORTED=1
STEAM_GAMESCOPE_HAS_TEARING_SUPPORT=1
STEAM_GAMESCOPE_TEARING_SUPPORTED=1
STEAM_GAMESCOPE_VRR_SUPPORTED=1
STEAM_DISPLAY_REFRESH_LIMITS=60,72,120,144

# NVIDIA-specific
VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/nvidia_icd.json
__GLX_VENDOR_LIBRARY_NAME=nvidia
__GL_SHADER_DISK_CACHE=1
WLR_NO_HARDWARE_CURSORS=1
__GL_GSYNC_ALLOWED=1
__GL_VRR_ALLOWED=1
__GL_THREADED_OPTIMIZATIONS=1
__GL_YIELD=NOTHING
```

## Troubleshooting

### Steam won't launch
1. Run `wizado info` to check dependencies
2. Ensure Steam is installed: `pacman -S steam`
3. Check logs: `wizado logs`

### Poor performance
1. Install GameMode: `sudo pacman -S gamemode lib32-gamemode`
2. Enable MangoHUD in settings to monitor performance
3. Try different FSR settings

### Controller not detected
1. Ensure you're in the `input` group: `groups | grep input`
2. If not: `sudo usermod -aG input $USER` then log out/in
3. Check detection: `wizado info | grep -i controller`

### NVIDIA issues
1. Ensure nvidia-utils is installed: `pacman -S nvidia-utils lib32-nvidia-utils`
2. Check driver version: `nvidia-smi`
3. Try disabling color management in gamescope (already default)

## License

MIT

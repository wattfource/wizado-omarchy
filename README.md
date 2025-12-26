# Wizado

Steam gaming launcher for Hyprland on Arch Linux (Omarchy).

Launches Steam in a fullscreen gamescope session on a dedicated workspace.

## How It Works

When you press `Super + Shift + S`:
1. Finds an empty workspace (or uses workspace 10)
2. Switches to that workspace
3. Launches gamescope + Steam in fullscreen
4. When you exit Steam, returns to your original workspace

All while Hyprland keeps running. Simple, reliable, works with NVIDIA.

## Features

- One-key launch with `Super + Shift + S`
- Force-quit with `Super + Shift + Q`
- TUI configuration via `wizado-config`
- Auto-detects display resolution
- Launches on empty workspace
- Returns to original workspace on exit
- Suspends hypridle during gaming
- FSR upscaling support
- Frame rate limiting
- VRR/Adaptive Sync
- NVIDIA GPU auto-detection

## Requirements

- Arch Linux with Hyprland
- Steam
- gamescope
- gum (for TUI config)
- jq (for waybar config)
- bc (for FSR calculations)
- A Nerd Font (for waybar icon)

## Installation

### One-Liner (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/REPLACE_ME/wizado/main/install.sh | bash
```

Or with wget:

```bash
wget -qO- https://raw.githubusercontent.com/REPLACE_ME/wizado/main/install.sh | bash
```

### Manual Installation

```bash
git clone https://github.com/REPLACE_ME/wizado.git
cd wizado
./scripts/setup.sh
```

### AUR (Coming Soon)

```bash
yay -S wizado
wizado setup
```

## License

Wizado requires a valid license to run. On first launch, you'll be prompted to enter your license key.

- **Price:** $5 for 5 machines
- **Get a license:** [wizado.app](https://wizado.app)

The license TUI will appear automatically when you press the launch keybind or click the waybar icon.

## Usage

| Command | Action |
|---------|--------|
| `wizado` | Launch Steam (requires license) |
| `wizado-launch` | Launch with license prompt |
| `wizado-config` | Configure settings & license via TUI |
| `Super + Shift + S` | Launch Steam (keybind) |
| `Super + Shift + Q` | Force-quit Steam + gamescope |

### Waybar Module

After installation, a wizado icon () appears in your waybar:

- **Left-click:** Launch Steam (or enter license if not activated)
- **Right-click:** Open settings TUI

If the icon doesn't appear, ensure you have a Nerd Font and restart waybar:
```bash
pkill waybar && waybar &
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

## Why Nested Mode?

After extensive testing, running gamescope nested inside Hyprland is the only reliable method for NVIDIA GPUs. The performance difference vs "true" Deck mode is negligible (1-5%).

**Nested mode benefits:**
- Stable, no visual glitches
- Easy to exit (just close Steam)
- Switch workspaces normally with Ctrl+Alt+arrows
- Full gamescope features (cursor lock, FSR, frame limiting)
- Works reliably with NVIDIA

---

## What Doesn't Work (Failure History)

### ❌ TTY3 Mode with NVIDIA (Visual Glitches)

**Goal:** Run gamescope directly on TTY3 (like Steam Deck) for zero compositor overhead.

**Results:**
- **Severe flickering** - Screen flickers constantly while Steam is running
- **Color corruption** - Colors become distorted, oversaturated
- **HDR state leak** - After returning to Hyprland, colors remain corrupted
- **Mouse glitches** - Visual artifacts follow cursor movement

**Cause:** NVIDIA's proprietary driver has poor support for VT switching when gamescope uses its DRM backend.

### ❌ Direct DRM Gamescope (Seat Conflicts)

```bash
gamescope --backend drm -- steam -gamepadui
# → "Could not take control of session: Device or resource busy"
```

Hyprland owns the DRM master and logind seat. Only one compositor can control the GPU at a time.

### ❌ Steam Without Gamescope (Ultrawide Issues)

Running `steam -gamepadui` directly works, but:
- Mouse constrained to wrong area on ultrawide monitors
- Games render fullscreen but input only works in 16:9/4:3 region

Gamescope fixes this by handling ultrawide scaling.

---

## Technical Notes

### Gamescope Flags Used

```bash
-W/-H                      # Output resolution (your monitor)
-w/-h                      # Render resolution (for FSR)
-f                         # Fullscreen
-e                         # Steam integration
--force-windows-fullscreen # Better game compatibility
--disable-color-management # CRITICAL for NVIDIA - prevents HDR leaking
--prefer-vk-device XXXX:YYYY  # Force specific GPU
```

### NVIDIA Environment Variables

```bash
VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/nvidia_icd.json
__GLX_VENDOR_LIBRARY_NAME=nvidia
__GL_SHADER_DISK_CACHE=1
WLR_NO_HARDWARE_CURSORS=1
XCURSOR_SIZE=24
```

### Using Gamemode with Individual Games

Gamemode (Feral Interactive) optimizes CPU/GPU while gaming. Enable per-game:

1. Right-click game in Steam → Properties
2. Launch Options: `gamemoderun %command%`

Don't wrap gamescope with gamemoderun—it should optimize the game, not the compositor.

## Uninstall

```bash
./scripts/remove.sh
```

## Glossary

See [GLOSSARY.md](GLOSSARY.md) for detailed explanations of technical terms:
- Compositor, Wayland, X11, XWayland
- DRM, KMS, DRM Master
- Seat, logind, seatd, Session
- TTY/VT, getty
- Gamescope, FSR, Proton, DXVK, Vulkan
- NVIDIA-specific settings and workarounds
- Environment variables reference

## License

MIT

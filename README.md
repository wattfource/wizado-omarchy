# Wizado 🧙‍♂️

**Wizado** is an **Omarchy-only** "Couch Mode" companion for Hyprland on Arch Linux.

It launches Steam in a dedicated `gamescope` session and wires up Omarchy’s Hyprland + Waybar integration.

## Features

- 🎮 **Seamless Launcher:** Launch Steam Big Picture / Gamepad UI in an optimized `gamescope` window.
- ⚡ **Performance Mode:** Auto-applies `gamemode`, real-time scheduling (`cap_sys_nice`), and GPU optimizations (AMD/NVIDIA).
- 🖥️ **Smart Resolution:** Auto-detects native resolution or allows TUI-based configuration for upscaling (e.g., render at 1080p, output at 4K).
- ⚙️ **TUI Settings:** Built-in terminal menu for configuration.
- 🍫 **Waybar Integration:** Adds a gamepad icon to Waybar for mouse-driven launching and status.
- ⌨️ **Hyprland Bindings:**
  - `Super + Shift + S`: Launch Steam (Nested mode)
  - `Super + Alt + S`: Launch Steam (Performance mode)
  - `Super + Shift + R`: Exit/Kill Session

## Requirements (by design)

Wizado is designed **only** for:
- **Omarchy (Arch Linux)**
- **Hyprland**
- **Limine + UKI** boot flow (Omarchy default), with kernel cmdline in `/boot/limine.conf`

## Installation

### Local Install (Development / Manual)

If you have cloned the repository and want to install it directly:

```bash
# Run the setup script directly
./scripts/setup.sh
```

This will:
1. Check and install dependencies (Steam, gamescope, drivers, etc.).
2. Configure permissions (user groups, udev rules).
3. Install launcher scripts to `~/.local/share/steam-launcher`.
4. Add Hyprland bindings and Waybar configuration.

### Arch Package (Recommended)

To install it as a system package (cleaner removal/updates):

```bash
# Build and install using makepkg
makepkg -si
```

## Updating

To update your installation after pulling new changes:

**Manual Method:**
```bash
git pull
./scripts/setup.sh
```

**Package Method:**
```bash
git pull
makepkg -si
```

## Submitting to AUR

Instructions for the package maintainer to submit/update the AUR package.

1. **Update Version:**
   Edit `PKGBUILD` and update `pkgver` and `pkgrel` if necessary.

2. **Generate Checksums:**
   If source files changed (though currently it skips sums for local dev), update them:
   ```bash
   updpkgsums
   ```

3. **Generate .SRCINFO:**
   This is required for the AUR to parse metadata.
   ```bash
   makepkg --printsrcinfo > .SRCINFO
   ```

4. **Test Build:**
   Ensure it builds in a clean environment.
   ```bash
   makepkg -f
   ```

5. **Push to AUR:**
   (Assuming you have the AUR git repo cloned)
   ```bash
   # Copy PKGBUILD and .SRCINFO to your AUR repo folder
   cp PKGBUILD .SRCINFO /path/to/wizado-aur/
   cd /path/to/wizado-aur/
   git add PKGBUILD .SRCINFO
   git commit -m "Update to v0.1.x"
   git push
   ```

## Usage

- **Launch:** Press `Super + Shift + S` or click the  icon in Waybar.
- **Settings:** Click the Waybar icon to open the menu, or select "Settings" before launching.
- **Exit:** Press `Super + Shift + R` or use the "Exit" option in the Steam power menu.

### Enable Performance Mode (Super+Alt+S)

To make `Super + Alt + S` truly “exclusive” (**Performance mode**: separate VT + DRM backend, Hyprland not in the render path), run:

```bash
wizado enable-tty
```

Then reboot. This installs:
- A sudoers drop-in for passwordless `openvt/chvt`
- `nvidia-drm.modeset=1` into `/boot/limine.conf` (required for NVIDIA DRM mode)

### Select the Performance GPU (multi-GPU systems)

If your system has **more than one GPU** (e.g. AMD iGPU + NVIDIA dGPU), performance mode must know which **DRM/KMS device** to use.

Open the menu (Waybar gamepad icon) and set:
- **Settings → Performance GPU**

This saves `PERFORMANCE_DRM_DEVICE` (and `PERFORMANCE_VK_DEVICE`) into `~/.gaming-mode.conf`, and performance mode will always use that GPU via `WLR_DRM_DEVICES`.

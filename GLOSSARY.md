# Wizado Glossary

Technical terms, concepts, and technologies relevant to this project.

---

## Display & Graphics Stack

### Compositor

**What it is:** A program that combines (composites) multiple visual elements into a final image displayed on screen. It manages windows, handles transparency, animations, and sends the final frame to the GPU.

**How it works:** Applications render to off-screen buffers. The compositor collects these buffers, arranges them according to window positions, applies effects, and produces a single image for the display.

**Why it matters:** Hyprland is a compositor. Gamescope is also a compositor. Only one compositor can directly control the display at a time (unless nested).

**Examples:** Hyprland, Sway, KWin, Mutter, gamescope, cage

---

### Wayland

**What it is:** A modern display server protocol that replaced X11 on Linux. Defines how applications communicate with a compositor.

**How it works:** Instead of a separate display server (like X11's Xorg), Wayland compositors are the display server. Apps talk directly to the compositor via the Wayland protocol. More secure (apps can't spy on each other's windows) and more efficient (fewer copies of image data).

**Why it matters:** Hyprland is a Wayland compositor. Gamescope can run as either a Wayland compositor (DRM mode) or a Wayland client (nested inside another compositor).

**Key environment variable:** `WAYLAND_DISPLAY` - tells apps which Wayland compositor to connect to.

---

### X11 / Xorg

**What it is:** The old display server protocol (1984). Still widely used because many applications and games only support X11.

**How it works:** A separate Xorg server process manages the display. Apps connect to it via the X11 protocol. The server handles drawing, input, and window management.

**Why it matters:** Many Windows games (via Proton) use X11. That's why XWayland exists.

---

### XWayland

**What it is:** An X11 server that runs as a Wayland client. Allows X11 applications to run on Wayland desktops.

**How it works:** XWayland pretends to be an X11 server to legacy apps, but renders its output to a Wayland surface. The Wayland compositor then displays it alongside native Wayland apps.

**Why it matters:** Steam and most games run through XWayland. Gamescope provides its own XWayland server for games, which is why you see `:2` in logs (gamescope's X display).

---

### DRM (Direct Rendering Manager)

**What it is:** A Linux kernel subsystem that provides an API for interacting with GPUs. Not "Digital Rights Management."

**How it works:** DRM exposes GPU resources (framebuffers, planes, CRTCs) to userspace. Compositors use DRM to display frames directly on monitors. DRM also handles mode setting (resolution, refresh rate).

**Why it matters:** When gamescope runs in "DRM mode" (backend), it talks directly to the GPU via DRM, bypassing any other compositor. This is the most efficient path but requires exclusive GPU access.

**Related:** libdrm, DRM master, DRM lease

---

### DRM Master

**What it is:** A file descriptor that grants exclusive control over a DRM device (GPU output).

**How it works:** Only one process can be DRM master at a time. The master can set display modes, flip buffers to the screen, etc. Non-masters can render but not display.

**Why it matters:** Hyprland holds DRM master while running. Gamescope can't take DRM master without Hyprland releasing it first. This is why "Device or resource busy" errors occur.

---

### KMS (Kernel Mode Setting)

**What it is:** Part of DRM that handles display configuration (resolution, refresh rate, connector selection) in the kernel.

**How it works:** Before KMS, display modes were set in userspace (fragile, slow). KMS moves this to the kernel for reliability. Compositors use KMS via DRM to configure displays.

**Why it matters:** Gamescope uses KMS when running in DRM mode to configure your monitor directly.

---

### Framebuffer

**What it is:** A block of memory representing a complete screen image.

**How it works:** The compositor renders the final composited image into a framebuffer. The GPU scans out (reads) this framebuffer to send to the display. Double/triple buffering uses multiple framebuffers to prevent tearing.

**Why it matters:** Gamescope manages framebuffers for games, handling the rendering → display pipeline.

---

### VSync (Vertical Synchronization)

**What it is:** Synchronizing frame presentation with the monitor's refresh cycle.

**How it works:** Instead of displaying frames immediately (which can cause tearing), VSync waits for the monitor's vertical blank interval to swap buffers.

**Why it matters:** Gamescope handles VSync for games, providing smooth frame pacing even if the game's internal VSync is broken.

---

### VRR (Variable Refresh Rate)

**What it is:** Technology allowing the monitor to adjust its refresh rate to match the GPU's frame output (G-Sync, FreeSync, Adaptive Sync).

**How it works:** Instead of fixed 60Hz/144Hz, the monitor refreshes whenever a new frame is ready. Eliminates tearing without VSync's input lag.

**Why it matters:** Gamescope supports VRR via the `--adaptive-sync` flag. Requires VRR-capable monitor and GPU support.

---

### HDR (High Dynamic Range)

**What it is:** Wider color gamut and higher brightness range than SDR (Standard Dynamic Range).

**How it works:** HDR content uses 10+ bits per color channel and specific color spaces (like HDR10, PQ curve). Requires HDR-capable display and proper color management throughout the stack.

**Why it matters:** Gamescope can enable HDR for games, but NVIDIA's implementation has bugs. HDR state can "leak" to other displays/VTs, corrupting colors. We disable it with `--disable-color-management`.

---

## Session & Seat Management

### Seat

**What it is:** A logical grouping of input/output devices that a user can interact with (monitor, keyboard, mouse, GPU).

**How it works:** A seat represents "one physical place where a user sits." Multi-seat systems allow multiple users on one machine. Most desktops have one seat (`seat0`).

**Why it matters:** Compositors must "take control" of a seat to access the GPU and input devices. Only one compositor per seat.

---

### logind (systemd-logind)

**What it is:** The systemd service that manages user sessions and seats.

**How it works:** When you log in, logind creates a session and assigns it to a seat. logind brokers access to devices—compositors ask logind for permission to use the GPU/input. It handles VT switching.

**Why it matters:** Gamescope uses logind to request seat access. "Device or resource busy" errors come from logind saying another process (Hyprland) already controls the seat.

**Environment:** `LIBSEAT_BACKEND=logind`

---

### seatd

**What it is:** An alternative to logind for seat management. Simpler, doesn't require systemd.

**How it works:** A small daemon that manages seat access. Compositors connect to seatd's socket to request device access.

**Why it matters:** We tried using seatd as an alternative to logind but hit permission issues. Most Arch systems use logind anyway.

---

### Session

**What it is:** A logind concept representing a user's login. Each VT login creates a session.

**How it works:** Sessions track which user is logged in, which seat they're on, and which processes belong to them. Compositors run within sessions and request device access through them.

**Why it matters:** Switching VTs involves session activation/deactivation. The active session's compositor has GPU access.

---

### TTY / VT (Virtual Terminal)

**What it is:** Text-based consoles provided by the Linux kernel. Ctrl+Alt+F1-F7 switch between them.

**How it works:** The kernel provides multiple virtual consoles (usually 6). Each can run a getty (login prompt), a compositor, or other programs. Only one VT is active (displayed) at a time.

**Why it matters:** Hyprland runs on VT1. We attempted to run gamescope on VT3. VT switching with NVIDIA causes problems.

**Commands:** `chvt N` (switch to VT N), `tty` (show current TTY)

---

### getty / agetty

**What it is:** Programs that manage login prompts on TTYs.

**How it works:** systemd starts getty services for each TTY. Getty displays "hostname login:" and handles authentication, then spawns your shell.

**Why it matters:** TTY3 auto-login is configured by overriding the getty service to use `--autologin username`.

---

## Gaming Technologies

### Gamescope

**What it is:** Valve's micro-compositor designed for gaming. Used on Steam Deck.

**How it works:** Gamescope creates an isolated Wayland+XWayland environment for games. It handles resolution scaling (FSR), frame limiting, HDR, VRR, and Steam Deck-specific features. Can run nested (inside another compositor) or standalone (DRM mode).

**Why it matters:** Gamescope solves many gaming problems: ultrawide mouse issues, frame pacing, FSR upscaling, cursor capture. It's essential for a Steam Deck-like experience.

**Key flags:**
- `-W/-H`: Output (display) resolution
- `-w/-h`: Render (game) resolution
- `-f`: Fullscreen
- `-e`: Steam integration
- `--prefer-vk-device`: Select GPU

---

### FSR (FidelityFX Super Resolution)

**What it is:** AMD's upscaling technology. Renders games at lower resolution, then upscales with a sharpening algorithm.

**How it works:** Game renders at e.g., 1080p. FSR upscales to 1440p/4K with spatial algorithms (FSR 1.0) or temporal data (FSR 2.0+). Gains performance at some quality cost.

**Why it matters:** Gamescope has FSR built-in. Set render resolution lower than output resolution (`-w 1920 -h 1080 -W 2560 -H 1440 --filter fsr`).

**Quality presets:** Ultra (77%), Quality (67%), Balanced (59%), Performance (50%)

---

### Proton

**What it is:** Valve's compatibility layer for running Windows games on Linux.

**How it works:** Based on Wine. Translates Windows API calls to Linux equivalents. Includes DXVK (DirectX→Vulkan), VKD3D (DX12→Vulkan), and other compatibility tools.

**Why it matters:** Most Steam games are Windows-only. Proton makes them playable on Linux. Steam automatically uses Proton when needed.

---

### Wine

**What it is:** "Wine Is Not an Emulator." Compatibility layer that runs Windows applications on Linux/macOS.

**How it works:** Implements Windows DLLs and system calls on top of POSIX. No CPU emulation—runs Windows executables natively.

**Why it matters:** Proton is built on Wine. Understanding Wine helps debug game issues.

---

### DXVK

**What it is:** DirectX 9/10/11 to Vulkan translation layer.

**How it works:** Games call DirectX functions. DXVK intercepts these and translates to Vulkan API calls that Linux GPU drivers understand.

**Why it matters:** Part of Proton. Most Windows games use DirectX; DXVK makes them run on Linux GPUs.

---

### Vulkan

**What it is:** Modern, low-level graphics API (successor to OpenGL).

**How it works:** Provides direct GPU control with minimal driver overhead. Cross-platform (Windows, Linux, Android, etc.).

**Why it matters:** Linux gaming relies heavily on Vulkan. DXVK/VKD3D translate DirectX to Vulkan. Native Linux games often use Vulkan directly.

---

### Vulkan Layers

**What it is:** Intercepting libraries that sit between apps and the Vulkan driver.

**How it works:** Layers can inspect/modify Vulkan calls for debugging, profiling, or adding features. Multiple layers can stack.

**Known layers:**
- MangoHud (`VK_LAYER_MANGOHUD_overlay`)
- Steam Fossilize (`VK_LAYER_VALVE_steam_fossilize`)
- Gamescope WSI (`VK_LAYER_FROG_gamescope_wsi`)

**Why it matters:** Layer conflicts cause crashes. Sometimes need to disable layers via environment variables.

---

### MangoHud

**What it is:** Performance overlay for Vulkan/OpenGL games (like MSI Afterburner/RivaTuner on Windows).

**How it works:** Vulkan layer that displays FPS, frame time, CPU/GPU usage, temperatures, etc. overlaid on the game.

**Why it matters:** Useful for monitoring performance. Enable via `MANGOHUD=1`. Can conflict with other layers.

---

### GameMode

**What it is:** Feral Interactive's daemon that optimizes system settings while gaming.

**How it works:** When a game starts, GameMode can: set CPU governor to performance, adjust GPU power profile, change process niceness/priority, disable screen savers.

**Usage:** `gamemoderun %command%` in Steam launch options.

**Why it matters:** Provides modest performance improvements. Apply to games, not to gamescope itself.

---

### Steam Deck UI / gamepadui

**What it is:** Steam's console-like interface designed for controllers and the Steam Deck.

**How it works:** Optimized for gamepad navigation, full-screen usage, and integrated gaming. Lighter than the desktop Steam client.

**Launch flag:** `steam -gamepadui`

**Why it matters:** Preferred UI for wizado—designed for the exact use case of gaming without a desktop.

---

### Big Picture Mode / tenfoot

**What it is:** The older Steam console interface (pre-Deck).

**How it works:** Full-screen, controller-friendly interface. Heavier than gamepadui.

**Launch flag:** `steam -tenfoot`

**Why it matters:** Legacy option. Use `-gamepadui` instead for better performance.

---

## NVIDIA-Specific

### NVIDIA Proprietary Driver

**What it is:** NVIDIA's closed-source Linux GPU driver.

**How it works:** Provides OpenGL, Vulkan, and CUDA support for NVIDIA GPUs. Interfaces with the kernel via NVIDIA's own kernel module (not the open-source nouveau).

**Why it matters:** Has quirks with Wayland, VT switching, and DRM backends. Many of wizado's issues are NVIDIA-specific.

---

### WLR_NO_HARDWARE_CURSORS

**What it is:** Environment variable that forces software cursor rendering.

**How it works:** Normally, the GPU renders the cursor in a dedicated hardware plane (efficient). This disables that, making the compositor render the cursor as part of the frame.

**Why it matters:** NVIDIA has hardware cursor bugs. Setting `WLR_NO_HARDWARE_CURSORS=1` fixes cursor glitches.

---

### VK_ICD_FILENAMES

**What it is:** Environment variable specifying which Vulkan driver to use.

**How it works:** Points to the ICD (Installable Client Driver) JSON file that tells the Vulkan loader which driver library to load.

**Why it matters:** On multi-GPU systems or to force NVIDIA: `VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/nvidia_icd.json`

---

### __GLX_VENDOR_LIBRARY_NAME

**What it is:** Environment variable selecting the GLX (OpenGL on X11) implementation.

**How it works:** Forces use of NVIDIA's GLX library instead of alternatives (like Mesa).

**Why it matters:** Set to `nvidia` to ensure OpenGL games use the NVIDIA driver.

---

## Processes & System

### setsid

**What it is:** Command that runs a program in a new session, detached from the terminal.

**How it works:** Creates a new session and process group. The process won't receive signals from the original terminal and survives if the parent dies.

**Why it matters:** Used to launch the deck session script so it survives Hyprland exiting.

---

### chvt

**What it is:** Command to switch virtual terminals.

**How it works:** `chvt N` switches to VT N. Requires root or appropriate permissions.

**Why it matters:** Used to switch from Hyprland (VT1) to gaming (VT3).

---

### pkill

**What it is:** Command to send signals to processes by name.

**How it works:** `pkill -9 steam` sends SIGKILL to all processes named "steam".

**Why it matters:** Used to force-quit Steam/gamescope when they don't respond.

---

### XDG_RUNTIME_DIR

**What it is:** Directory for user-specific runtime files (sockets, pipes, etc.).

**How it works:** Set by logind to `/run/user/UID`. Wayland compositors create their sockets here (e.g., `wayland-1`).

**Why it matters:** Must be set for Wayland apps to find the compositor. Critical for proper session setup.

---

## File Locations

| Path | Purpose |
|------|---------|
| `~/.config/wizado/config` | User configuration |
| `~/.cache/wizado/wizado.log` | Runtime logs |
| `/run/user/1000/` | XDG_RUNTIME_DIR (sockets, etc.) |
| `/run/seatd.sock` | seatd socket (if used) |
| `/usr/share/vulkan/icd.d/` | Vulkan driver manifests |
| `/etc/systemd/system/getty@ttyN.service.d/` | TTY auto-login overrides |

---

## Quick Reference: Environment Variables

```bash
# Wayland
WAYLAND_DISPLAY=wayland-1      # Which compositor to connect to
XDG_RUNTIME_DIR=/run/user/1000 # Runtime directory

# Seat management
LIBSEAT_BACKEND=logind         # Use logind for seat

# NVIDIA
VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/nvidia_icd.json
__GLX_VENDOR_LIBRARY_NAME=nvidia
WLR_NO_HARDWARE_CURSORS=1
__GL_SHADER_DISK_CACHE=1

# Gaming
MANGOHUD=1                     # Enable MangoHud overlay
SDL_VIDEO_MINIMIZE_ON_FOCUS_LOSS=0  # Don't minimize on focus loss

# Debugging
WAYLAND_DEBUG=1                # Wayland protocol logging
LIBSEAT_LOGLEVEL=debug         # Seat management logging
```



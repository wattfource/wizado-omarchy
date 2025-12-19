#!/usr/bin/env bash
set -Euo pipefail

# wizado setup: Install Steam gaming mode with gamescope for Hyprland
# Includes performance optimizations for maximum gaming performance

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

TARGET_DIR="${HOME}/.local/share/steam-launcher"
SWITCH_BIN="${TARGET_DIR}/enter-gamesmode"
RETURN_BIN="${TARGET_DIR}/leave-gamesmode"

UDEV_RULES_FILE="/etc/udev/rules.d/99-wizado-gaming.rules"
NVIDIA_XORG_CONF="/etc/X11/xorg.conf.d/20-nvidia-performance.conf"

# Find Hyprland bindings config
BINDINGS_CONFIG=""
for location in \
    "$HOME/.config/hypr/bindings.conf" \
    "$HOME/.config/hypr/keybinds.conf" \
    "$HOME/.config/hypr/hyprland.conf"; do
  if [[ -f "$location" ]]; then
    BINDINGS_CONFIG="$location"
    break
  fi
done

ADDED_BINDINGS=0
CREATED_TARGET_DIR=0
NEEDS_RELOGIN=0
NEEDS_REBOOT=0

# Track detected hardware
HAS_NVIDIA=false
HAS_AMD=false
HAS_INTEL=false
NVIDIA_VK_ID=""

rollback_changes() {
  [[ -f "$SWITCH_BIN" ]] && rm -f "$SWITCH_BIN"
  [[ -f "$RETURN_BIN" ]] && rm -f "$RETURN_BIN"
  if [[ "$CREATED_TARGET_DIR" -eq 1 ]] && [[ -d "$TARGET_DIR" ]]; then
    rmdir "$TARGET_DIR" 2>/dev/null || true
  fi

  if [[ "$ADDED_BINDINGS" -eq 1 ]] && [[ -n "$BINDINGS_CONFIG" ]] && [[ -f "$BINDINGS_CONFIG" ]]; then
    sed -i '/# Gaming Mode bindings - added by wizado/,/# End Gaming Mode bindings/d' "$BINDINGS_CONFIG"
  fi
}

die() {
  local msg="$1"
  warn "$msg"
  logger -t wizado "Installation failed: $msg" 2>/dev/null || true
  rollback_changes
  exit 1
}

validate_environment() {
  require_cmd pacman
  require_cmd hyprctl
  [[ -n "$BINDINGS_CONFIG" ]] || die "Could not find Hyprland bindings config"
  [[ -f "$BINDINGS_CONFIG" ]] || die "Bindings config not found: $BINDINGS_CONFIG"
  [[ -d "$HOME/.config/hypr" ]] || die "Hyprland config directory not found"
}

check_package() {
  pacman -Qi "$1" &>/dev/null
}

# ============================================================================
# Hardware Detection
# ============================================================================

detect_all_gpus() {
  log "Detecting GPUs..."
  
  if ! command -v lspci >/dev/null 2>&1; then
    warn "lspci not found, cannot detect GPUs"
    return 0
  fi
  
  local gpu_info
  gpu_info="$(lspci 2>/dev/null | grep -iE 'vga|3d|display' || true)"
  
  if echo "$gpu_info" | grep -iq nvidia; then
    HAS_NVIDIA=true
    # Get NVIDIA Vulkan device ID
    NVIDIA_VK_ID=$(lspci -nn | grep -i "NVIDIA" | grep -oP '\[10de:[0-9a-f]{4}\]' | head -n1 | tr -d '[]')
    log "  NVIDIA GPU detected: $NVIDIA_VK_ID"
  fi
  
  if echo "$gpu_info" | grep -iqE 'amd|radeon|advanced micro'; then
    HAS_AMD=true
    log "  AMD GPU detected"
  fi
  
  if echo "$gpu_info" | grep -iq intel; then
    HAS_INTEL=true
    local intel_type
    intel_type="$(detect_intel_gpu_type)"
    log "  Intel GPU detected: $intel_type"
  fi
  
  if ! $HAS_NVIDIA && ! $HAS_AMD && ! $HAS_INTEL; then
    warn "No recognized GPU detected"
  fi
}

detect_intel_gpu_type() {
  local has_arc=false has_igpu=false
  for card_device in /sys/class/drm/card*/device/driver; do
    [[ -e "$card_device" ]] || continue
    [[ -L "$card_device" ]] || continue
    local driver_name
    driver_name="$(basename "$(readlink -f "$card_device" 2>/dev/null || true)")"
    case "$driver_name" in
      xe) has_arc=true ;;
      i915) has_igpu=true ;;
    esac
  done
  if $has_arc && $has_igpu; then echo "both"
  elif $has_arc; then echo "arc"
  elif $has_igpu; then echo "igpu"
  else echo "none"
  fi
}

# ============================================================================
# Performance Optimizations
# ============================================================================

setup_cpu_performance() {
  log "Configuring CPU performance mode..."
  
  # Check current governor
  local current_gov
  current_gov=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null || echo "unknown")
  log "  Current CPU governor: $current_gov"
  
  if [[ "$current_gov" == "performance" ]]; then
    log "  CPU already in performance mode"
  else
    log "  Setting CPU governor to performance"
  fi
  
  # Create systemd service for persistent CPU performance mode
  local service_file="/etc/systemd/system/wizado-cpu-performance.service"
  
  if [[ ! -f "$service_file" ]]; then
    echo ""
    warn "CPU governor needs to be set to 'performance' for best gaming"
    confirm_or_die "Create systemd service for CPU performance mode?"
    
    sudo tee "$service_file" > /dev/null <<'EOF'
[Unit]
Description=Set CPU governor to performance for gaming
After=multi-user.target

[Service]
Type=oneshot
ExecStart=/bin/bash -c 'for cpu in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do echo performance > "$cpu" 2>/dev/null || true; done'
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF
    
    sudo systemctl daemon-reload
    sudo systemctl enable wizado-cpu-performance.service
    sudo systemctl start wizado-cpu-performance.service
    log "  CPU performance service installed and started"
  else
    log "  CPU performance service already exists"
    # Ensure it's running
    sudo systemctl start wizado-cpu-performance.service 2>/dev/null || true
  fi
}

setup_nvidia_performance() {
  $HAS_NVIDIA || return 0
  
  log "Configuring NVIDIA GPU performance..."
  
  # 1. Enable persistence mode (keeps GPU initialized)
  if command -v nvidia-smi >/dev/null 2>&1; then
    log "  Enabling NVIDIA persistence mode"
    sudo nvidia-smi -pm 1 2>/dev/null || warn "Could not enable persistence mode"
    
    # Create systemd service for persistent nvidia-persistenced
    local nvidia_persist_service="/etc/systemd/system/nvidia-persistenced.service"
    if [[ ! -f "$nvidia_persist_service" ]] && ! systemctl is-enabled nvidia-persistenced >/dev/null 2>&1; then
      # Check if nvidia-persistenced exists
      if command -v nvidia-persistenced >/dev/null 2>&1; then
        sudo systemctl enable nvidia-persistenced 2>/dev/null || true
        sudo systemctl start nvidia-persistenced 2>/dev/null || true
        log "  NVIDIA persistence daemon enabled"
      fi
    fi
  fi
  
  # 2. Set PowerMizer to maximum performance
  if command -v nvidia-settings >/dev/null 2>&1; then
    log "  Setting PowerMizer to Prefer Maximum Performance"
    nvidia-settings -a '[gpu:0]/GpuPowerMizerMode=1' >/dev/null 2>&1 || true
  fi
  
  # 3. Create Xorg config for persistent PowerMizer setting (affects Wayland too via nvidia-settings)
  if [[ ! -f "$NVIDIA_XORG_CONF" ]]; then
    echo ""
    warn "NVIDIA PowerMizer can be set permanently via Xorg config"
    confirm_or_die "Create NVIDIA performance config?"
    
    sudo mkdir -p "$(dirname "$NVIDIA_XORG_CONF")"
    sudo tee "$NVIDIA_XORG_CONF" > /dev/null <<'EOF'
# NVIDIA Performance Settings for Gaming
# Created by wizado

Section "Device"
    Identifier     "Nvidia Card"
    Driver         "nvidia"
    VendorName     "NVIDIA Corporation"
    
    # PowerMizer: 1 = Prefer Maximum Performance
    Option         "RegistryDwords" "PowerMizerEnable=0x1; PerfLevelSrc=0x2222; PowerMizerDefault=0x1; PowerMizerDefaultAC=0x1"
    
    # Enable triple buffering for better frame pacing
    Option         "TripleBuffer" "True"
EndSection
EOF
    log "  NVIDIA Xorg config created"
    NEEDS_REBOOT=1
  else
    log "  NVIDIA Xorg config already exists"
  fi
  
  # 4. Check and report current GPU state
  if command -v nvidia-smi >/dev/null 2>&1; then
    local gpu_info
    gpu_info=$(nvidia-smi --query-gpu=gpu_name,power.draw,clocks.gr,clocks.mem --format=csv,noheader 2>/dev/null || true)
    if [[ -n "$gpu_info" ]]; then
      log "  Current GPU state: $gpu_info"
    fi
  fi
}

setup_amd_performance() {
  $HAS_AMD || return 0
  
  log "Configuring AMD GPU performance..."
  
  # AMD GPUs use power_dpm_force_performance_level
  local amd_perf_found=false
  for card in /sys/class/drm/card*/device/power_dpm_force_performance_level; do
    if [[ -f "$card" ]]; then
      amd_perf_found=true
      local current_level
      current_level=$(cat "$card" 2>/dev/null || echo "unknown")
      log "  Current AMD performance level: $current_level"
    fi
  done
  
  if ! $amd_perf_found; then
    log "  AMD performance control not available (may be using different driver)"
    return 0
  fi
}

setup_udev_rules() {
  log "Setting up udev rules for performance control..."
  
  if [[ -f "$UDEV_RULES_FILE" ]]; then
    log "  Udev rules already exist, updating..."
  fi
  
  echo ""
  warn "Udev rules allow gaming mode to control CPU/GPU performance without sudo"
  confirm_or_die "Install udev rules for performance control?"
  
  sudo tee "$UDEV_RULES_FILE" > /dev/null <<'EOF'
# Wizado Gaming Performance Rules
# Allow users in video/games group to control CPU/GPU performance

# CPU governor control
KERNEL=="cpu[0-9]*", SUBSYSTEM=="cpu", ACTION=="add", RUN+="/bin/chmod 666 /sys/devices/system/cpu/%k/cpufreq/scaling_governor"

# AMD GPU performance control
KERNEL=="card[0-9]", SUBSYSTEM=="drm", DRIVERS=="amdgpu", ACTION=="add", RUN+="/bin/chmod 666 /sys/class/drm/%k/device/power_dpm_force_performance_level"

# Intel GPU frequency control (i915)
KERNEL=="card[0-9]", SUBSYSTEM=="drm", DRIVERS=="i915", ACTION=="add", RUN+="/bin/chmod 666 /sys/class/drm/%k/gt_boost_freq_mhz"
KERNEL=="card[0-9]", SUBSYSTEM=="drm", DRIVERS=="i915", ACTION=="add", RUN+="/bin/chmod 666 /sys/class/drm/%k/gt_min_freq_mhz"
KERNEL=="card[0-9]", SUBSYSTEM=="drm", DRIVERS=="i915", ACTION=="add", RUN+="/bin/chmod 666 /sys/class/drm/%k/gt_max_freq_mhz"
EOF
  
  sudo udevadm control --reload-rules
  sudo udevadm trigger --subsystem-match=cpu --subsystem-match=drm
  
  log "  Udev rules installed"
}

save_hardware_config() {
  # Save detected hardware configuration for the launcher scripts
  local config_file="${TARGET_DIR}/hardware.conf"
  
  mkdir -p "$TARGET_DIR"
  
  cat > "$config_file" <<EOF
# Wizado Hardware Configuration
# Auto-generated by setup.sh on $(date -Is)
# Re-run 'wizado setup' to update after hardware changes

HAS_NVIDIA=$HAS_NVIDIA
HAS_AMD=$HAS_AMD
HAS_INTEL=$HAS_INTEL
NVIDIA_VK_ID="$NVIDIA_VK_ID"
EOF
  
  log "Hardware config saved to $config_file"
}

# ============================================================================
# Dependency Installation
# ============================================================================

ensure_multilib_enabled() {
  if grep -q "^\[multilib\]" /etc/pacman.conf 2>/dev/null; then
    log "Multilib repository: enabled"
    return 0
  fi

  warn "Multilib repository NOT enabled (required for Steam 32-bit libraries)"
  confirm_or_die "Enable multilib in /etc/pacman.conf?"

  sudo cp /etc/pacman.conf "/etc/pacman.conf.backup.$(date +%Y%m%d%H%M%S)" || die "Failed to backup pacman.conf"
  sudo sed -i '/^#\[multilib\]/,/^#Include/ s/^#//' /etc/pacman.conf || die "Failed to enable multilib"

  if ! grep -q "^\[multilib\]" /etc/pacman.conf 2>/dev/null; then
    die "Multilib enablement failed"
  fi

  log "Refreshing package database..."
  sudo pacman -Syy || die "Failed to refresh package database"
}

check_steam_dependencies() {
  log "Checking Steam dependencies..."

  # Refresh package database
  sudo pacman -Syy || die "Failed to refresh package database"

  local -a missing_deps=()

  # Core Steam dependencies
  local -a core_deps=(
    "steam"
    "lib32-vulkan-icd-loader"
    "vulkan-icd-loader"
    "lib32-mesa"
    "mesa"
    "mesa-utils"
    "lib32-systemd"
    "lib32-glibc"
    "lib32-gcc-libs"
    "lib32-libx11"
    "lib32-libxss"
    "lib32-alsa-plugins"
    "lib32-libpulse"
    "lib32-openal"
    "lib32-nss"
    "lib32-libcups"
    "lib32-sdl2"
    "lib32-freetype2"
    "lib32-fontconfig"
    "ttf-liberation"
    "xdg-user-dirs"
    "pciutils"
  )

  # GPU-specific drivers
  local -a gpu_deps=()
  
  if $HAS_NVIDIA; then
    gpu_deps+=(
      "nvidia-utils"
      "lib32-nvidia-utils"
      "nvidia-settings"
      "libva-nvidia-driver"
    )
  fi
  
  if $HAS_AMD; then
    gpu_deps+=(
      "vulkan-radeon"
      "lib32-vulkan-radeon"
      "libva-mesa-driver"
      "lib32-libva-mesa-driver"
    )
  fi
  
  if $HAS_INTEL; then
    local intel_type
    intel_type="$(detect_intel_gpu_type)"
    if [[ "$intel_type" == "arc" ]]; then
      gpu_deps+=(
        "vulkan-intel"
        "lib32-vulkan-intel"
        "intel-compute-runtime"
        "intel-gpu-tools"
      )
    else
      gpu_deps+=(
        "vulkan-intel"
        "lib32-vulkan-intel"
        "intel-media-driver"
        "libva-intel-driver"
        "lib32-libva-intel-driver"
        "intel-compute-runtime"
        "intel-gpu-tools"
      )
    fi
  fi
  
  # Fallback if no GPU detected
  if ! $HAS_NVIDIA && ! $HAS_AMD && ! $HAS_INTEL; then
    gpu_deps+=(
      "vulkan-radeon"
      "lib32-vulkan-radeon"
      "vulkan-intel"
      "lib32-vulkan-intel"
    )
  fi

  # Common Vulkan tools
  gpu_deps+=("vulkan-tools" "vulkan-mesa-layers")

  # Check core dependencies
  for dep in "${core_deps[@]}"; do
    check_package "$dep" || missing_deps+=("$dep")
  done

  # Check GPU dependencies
  for dep in "${gpu_deps[@]}"; do
    check_package "$dep" || missing_deps+=("$dep")
  done

  if ((${#missing_deps[@]})); then
    echo ""
    log "Missing required packages (${#missing_deps[@]}):"
    for dep in "${missing_deps[@]}"; do
      echo "  • $dep"
    done
    echo ""
    confirm_or_die "Install missing packages?"
    sudo pacman -S --needed "${missing_deps[@]}" || die "Failed to install dependencies"
    log "Dependencies installed"
  else
    log "All required dependencies installed"
  fi
}

install_recommended_packages() {
  local -a recommended=(
    "gamescope"
    "gamemode"
    "lib32-gamemode"
    "mangohud"
    "lib32-mangohud"
    "libcap"
    "python"
  )

  local -a missing=()
  for pkg in "${recommended[@]}"; do
    check_package "$pkg" || missing+=("$pkg")
  done

  if ((${#missing[@]})); then
    echo ""
    log "Recommended packages not installed:"
    for pkg in "${missing[@]}"; do
      echo "  • $pkg"
    done
    echo ""
    confirm_or_die "Install recommended packages?"
    sudo pacman -S --needed "${missing[@]}" || warn "Some packages failed to install"
  else
    log "All recommended packages installed"
  fi
}

# ============================================================================
# User Groups
# ============================================================================

check_user_groups() {
  local -a missing_groups=()

  groups | grep -q '\bvideo\b' || missing_groups+=("video")
  groups | grep -q '\binput\b' || missing_groups+=("input")

  if ((${#missing_groups[@]})); then
    echo ""
    warn "User '$USER' missing from groups: ${missing_groups[*]}"
    warn "Required for GPU access and controller support"
    confirm_or_die "Add user to groups?"

    local groups_csv
    groups_csv=$(IFS=,; echo "${missing_groups[*]}")
    sudo usermod -aG "$groups_csv" "$USER" || die "Failed to add user to groups"
    log "Added user to groups: $groups_csv"
    NEEDS_RELOGIN=1
  else
    log "User groups: OK"
  fi
}

# ============================================================================
# Gamescope Capability
# ============================================================================

maybe_grant_gamescope_cap() {
  command -v gamescope >/dev/null 2>&1 || return 0
  command -v getcap >/dev/null 2>&1 || return 0
  command -v setcap >/dev/null 2>&1 || return 0

  local gamescope_path
  gamescope_path="$(command -v gamescope)"

  if getcap "$gamescope_path" 2>/dev/null | grep -q 'cap_sys_nice'; then
    log "Gamescope already has cap_sys_nice"
    return 0
  fi

  echo ""
  warn "Gamescope can run with real-time priority if granted cap_sys_nice"
  warn "This reduces latency and improves frame pacing"
  confirm_or_die "Grant cap_sys_nice to gamescope?"

  sudo setcap 'cap_sys_nice+ep' "$gamescope_path" || warn "setcap failed"
  log "Granted cap_sys_nice to gamescope"
}

# ============================================================================
# Deploy Launchers
# ============================================================================

deploy_launchers() {
  if [[ ! -d "$TARGET_DIR" ]]; then
    mkdir -p "$TARGET_DIR" || die "Cannot create $TARGET_DIR"
    CREATED_TARGET_DIR=1
  fi

  local launcher_src="${SCRIPT_DIR}/launchers"

  if [[ ! -f "${launcher_src}/enter-gamesmode" ]] || [[ ! -f "${launcher_src}/leave-gamesmode" ]]; then
    die "Launcher scripts not found in ${launcher_src}"
  fi

  install -Dm755 "${launcher_src}/enter-gamesmode" "$SWITCH_BIN" || die "Failed to install enter-gamesmode"
  install -Dm755 "${launcher_src}/leave-gamesmode" "$RETURN_BIN" || die "Failed to install leave-gamesmode"

  log "Launchers installed to $TARGET_DIR"
}

# ============================================================================
# Hyprland Keybindings
# ============================================================================

configure_shortcuts() {
  log "Configuring Hyprland keybindings..."

  # Remove old bindings if they exist (allows re-run to update)
  if grep -q "# Gaming Mode bindings - added by wizado" "$BINDINGS_CONFIG" 2>/dev/null; then
    log "Removing old gaming mode bindings..."
    sed -i '/# Gaming Mode bindings - added by wizado/,/# End Gaming Mode bindings/d' "$BINDINGS_CONFIG"
  fi

  # Detect bind style
  local bind_style="bindd"
  if ! grep -q "^bindd[[:space:]]*=" "$BINDINGS_CONFIG" 2>/dev/null; then
    if grep -q "^bind[[:space:]]*=" "$BINDINGS_CONFIG" 2>/dev/null; then
      bind_style="bind"
    fi
  fi

  # Add bindings
  {
    echo ""
    echo "# Gaming Mode bindings - added by wizado"
    if [[ "$bind_style" == "bindd" ]]; then
      echo "bindd = SUPER SHIFT, S, Steam Gaming Mode, exec, $SWITCH_BIN"
      echo "bindd = SUPER SHIFT, R, Exit Gaming Mode, exec, $RETURN_BIN"
    else
      echo "bind = SUPER SHIFT, S, exec, $SWITCH_BIN"
      echo "bind = SUPER SHIFT, R, exec, $RETURN_BIN"
    fi
    echo "# End Gaming Mode bindings"
  } >> "$BINDINGS_CONFIG" || die "Failed to add keybindings"

  ADDED_BINDINGS=1

  # Reload Hyprland
  hyprctl reload >/dev/null 2>&1 || warn "Hyprland reload may have failed"
  log "Keybindings added to $BINDINGS_CONFIG"
}

# ============================================================================
# Main
# ============================================================================

usage() {
  cat <<'EOF'
wizado setup

Installs Steam gaming mode with gamescope for Hyprland (Omarchy).
Configures system for maximum gaming performance.

Options:
  --yes, -y     Non-interactive mode
  --dry-run     Print what would be done
  -h, --help    Show this help

What this does:
  • Installs Steam, gamescope, gamemode, and GPU drivers
  • Sets CPU governor to performance mode
  • Configures NVIDIA/AMD GPU for maximum performance
  • Sets up udev rules for user-level performance control
  • Adds Hyprland keybindings

Keybindings after install:
  Super + Shift + S   Launch Steam in gamescope
  Super + Shift + R   Exit gaming mode

Re-run this script after hardware changes to update configuration.
EOF
}

main() {
  if ! parse_common_flags "$@"; then
    usage
    exit 0
  fi

  require_omarchy

  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "  WIZADO - Steam Gaming Mode Setup"
  echo "════════════════════════════════════════════════════════════════"
  echo ""

  validate_environment
  log "Using bindings config: $BINDINGS_CONFIG"

  confirm_or_die "Install/update Steam gaming mode with maximum performance?"

  # Detect hardware first
  detect_all_gpus
  
  ensure_multilib_enabled
  check_steam_dependencies
  install_recommended_packages
  check_user_groups
  
  # Performance optimizations
  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "  PERFORMANCE OPTIMIZATIONS"
  echo "════════════════════════════════════════════════════════════════"
  echo ""
  
  setup_cpu_performance
  setup_nvidia_performance
  setup_amd_performance
  setup_udev_rules
  
  maybe_grant_gamescope_cap
  deploy_launchers
  save_hardware_config
  configure_shortcuts

  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "  INSTALLATION COMPLETE"
  echo "════════════════════════════════════════════════════════════════"
  echo ""
  echo "  Hardware detected:"
  $HAS_NVIDIA && echo "    • NVIDIA GPU: $NVIDIA_VK_ID"
  $HAS_AMD && echo "    • AMD GPU"
  $HAS_INTEL && echo "    • Intel GPU"
  echo ""
  echo "  Performance settings:"
  echo "    • CPU governor: performance"
  $HAS_NVIDIA && echo "    • NVIDIA PowerMizer: Maximum Performance"
  $HAS_AMD && echo "    • AMD performance level: high"
  echo ""
  echo "  Keybindings:"
  echo "    Super + Shift + S   Launch Steam in gamescope"
  echo "    Super + Shift + R   Exit gaming mode"
  echo ""
  echo "  Launchers: $TARGET_DIR"
  echo "  Config:    $BINDINGS_CONFIG"
  echo ""

  if [[ "$NEEDS_REBOOT" -eq 1 ]]; then
    echo "════════════════════════════════════════════════════════════════"
    echo "  ⚠  REBOOT RECOMMENDED"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "  Some performance settings require a reboot to take effect."
    echo ""
  elif [[ "$NEEDS_RELOGIN" -eq 1 ]]; then
    echo "════════════════════════════════════════════════════════════════"
    echo "  ⚠  LOG OUT REQUIRED"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "  User groups were updated. Log out and back in for changes"
    echo "  to take effect."
    echo ""
  fi
  
  echo "Re-run 'wizado setup' after hardware changes to update configuration."
  echo ""
}

main "$@"

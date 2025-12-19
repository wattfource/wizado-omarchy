#!/usr/bin/env bash
set -Euo pipefail

# wizado setup: Install Steam gaming mode with gamescope for Hyprland

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

TARGET_DIR="${HOME}/.local/share/steam-launcher"
SWITCH_BIN="${TARGET_DIR}/enter-gamesmode"
RETURN_BIN="${TARGET_DIR}/leave-gamesmode"

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
# GPU Detection
# ============================================================================

detect_gpu_vendor() {
  if ! command -v lspci >/dev/null 2>&1; then
    echo "unknown"
    return 0
  fi
  local g
  g="$(lspci 2>/dev/null | grep -iE 'vga|3d|display' || true)"
  if echo "$g" | grep -iq nvidia; then echo "nvidia"; return 0; fi
  if echo "$g" | grep -iqE 'amd|radeon|advanced micro'; then echo "amd"; return 0; fi
  if echo "$g" | grep -iq intel; then echo "intel"; return 0; fi
  echo "unknown"
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
  )

  # GPU-specific drivers
  local gpu_vendor
  gpu_vendor="$(detect_gpu_vendor)"
  log "Detected GPU: $gpu_vendor"

  local -a gpu_deps=()
  case "$gpu_vendor" in
    nvidia)
      gpu_deps=(
        "nvidia-utils"
        "lib32-nvidia-utils"
        "nvidia-settings"
        "libva-nvidia-driver"
      )
      ;;
    amd)
      gpu_deps=(
        "vulkan-radeon"
        "lib32-vulkan-radeon"
        "libva-mesa-driver"
        "lib32-libva-mesa-driver"
        "mesa-vdpau"
        "lib32-mesa-vdpau"
      )
      ;;
    intel)
      local intel_type
      intel_type="$(detect_intel_gpu_type)"
      if [[ "$intel_type" == "arc" ]]; then
        gpu_deps=(
          "vulkan-intel"
          "lib32-vulkan-intel"
          "intel-compute-runtime"
          "intel-gpu-tools"
        )
      else
        gpu_deps=(
          "vulkan-intel"
          "lib32-vulkan-intel"
          "intel-media-driver"
          "libva-intel-driver"
          "lib32-libva-intel-driver"
          "intel-compute-runtime"
          "intel-gpu-tools"
        )
      fi
      ;;
    *)
      log "GPU not detected, installing generic Vulkan drivers"
      gpu_deps=(
        "vulkan-radeon"
        "lib32-vulkan-radeon"
        "vulkan-intel"
        "lib32-vulkan-intel"
      )
      ;;
  esac

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

  # Check if bindings already exist
  if grep -q "# Gaming Mode bindings - added by wizado" "$BINDINGS_CONFIG" 2>/dev/null; then
    log "Gaming mode bindings already exist, skipping"
    return 0
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

Options:
  --yes, -y     Non-interactive mode
  --dry-run     Print what would be done
  -h, --help    Show this help

Keybindings after install:
  Super + Shift + S   Launch Steam in gamescope
  Super + Shift + R   Exit gaming mode
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

  confirm_or_die "Install Steam gaming mode with gamescope?"

  ensure_multilib_enabled
  check_steam_dependencies
  install_recommended_packages
  check_user_groups
  maybe_grant_gamescope_cap
  deploy_launchers
  configure_shortcuts

  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "  INSTALLATION COMPLETE"
  echo "════════════════════════════════════════════════════════════════"
  echo ""
  echo "  Keybindings:"
  echo "    Super + Shift + S   Launch Steam in gamescope"
  echo "    Super + Shift + R   Exit gaming mode"
  echo ""
  echo "  Launchers: $TARGET_DIR"
  echo "  Config:    $BINDINGS_CONFIG"
  echo ""

  if [[ "$NEEDS_RELOGIN" -eq 1 ]]; then
    echo "════════════════════════════════════════════════════════════════"
    echo "  ⚠  LOG OUT REQUIRED"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "  User groups were updated. Log out and back in for changes"
    echo "  to take effect."
    echo ""
  fi
}

main "$@"

#!/usr/bin/env bash
set -Euo pipefail

# wizado setup: Install Steam gaming launcher for Hyprland

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

LOCAL_BIN="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.config/wizado"

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
NEEDS_RELOGIN=0

# Track detected hardware
HAS_NVIDIA=false
HAS_AMD=false
HAS_INTEL=false
NVIDIA_VK_ID=""

die() {
  local msg="$1"
  warn "$msg"
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
    NVIDIA_VK_ID=$(lspci -nn | grep -i "NVIDIA" | grep -oP '\[10de:[0-9a-f]{4}\]' | head -n1 | tr -d '[]')
    log "  NVIDIA GPU detected: $NVIDIA_VK_ID"
  fi
  
  if echo "$gpu_info" | grep -iqE 'amd|radeon|advanced micro'; then
    HAS_AMD=true
    log "  AMD GPU detected"
  fi
  
  if echo "$gpu_info" | grep -iq intel; then
    HAS_INTEL=true
    log "  Intel GPU detected"
  fi
  
  if ! $HAS_NVIDIA && ! $HAS_AMD && ! $HAS_INTEL; then
    warn "No recognized GPU detected"
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

check_dependencies() {
  log "Checking dependencies..."

  sudo pacman -Syy || die "Failed to refresh package database"

  local -a missing_deps=()

  # Core dependencies
  local -a core_deps=(
    "steam"
    "gamescope"
    "gum"
    "bc"
    "lib32-vulkan-icd-loader"
    "vulkan-icd-loader"
    "lib32-mesa"
    "mesa"
    "pciutils"
  )

  # GPU-specific drivers
  if $HAS_NVIDIA; then
    core_deps+=(
      "nvidia-utils"
      "lib32-nvidia-utils"
    )
  fi
  
  if $HAS_AMD; then
    core_deps+=(
      "vulkan-radeon"
      "lib32-vulkan-radeon"
    )
  fi

  # Check dependencies
  for dep in "${core_deps[@]}"; do
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

install_optional_packages() {
  local -a optional=(
    "gamemode"
    "lib32-gamemode"
    "mangohud"
    "lib32-mangohud"
  )

  local -a missing=()
  for pkg in "${optional[@]}"; do
    check_package "$pkg" || missing+=("$pkg")
  done

  if ((${#missing[@]})); then
    echo ""
    log "Optional packages (performance monitoring, optimization):"
    for pkg in "${missing[@]}"; do
      echo "  • $pkg"
    done
    echo ""
    confirm_or_die "Install optional packages?"
    sudo pacman -S --needed "${missing[@]}" || warn "Some packages failed to install"
  else
    log "Optional packages already installed"
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
    warn "Required for GPU and input access"
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
# Deploy Scripts
# ============================================================================

deploy_scripts() {
  log "Installing wizado scripts..."
  
  mkdir -p "$LOCAL_BIN"
  mkdir -p "$CONFIG_DIR"

  local launcher_src="${SCRIPT_DIR}/launchers"

  # Install main launcher
  if [[ -f "${launcher_src}/wizado" ]]; then
    install -Dm755 "${launcher_src}/wizado" "${LOCAL_BIN}/wizado"
    log "  Installed: ${LOCAL_BIN}/wizado"
  else
    die "Launcher script not found: ${launcher_src}/wizado"
  fi

  # Install config TUI
  if [[ -f "${launcher_src}/wizado-config" ]]; then
    install -Dm755 "${launcher_src}/wizado-config" "${LOCAL_BIN}/wizado-config"
    log "  Installed: ${LOCAL_BIN}/wizado-config"
  fi

  # Install default config if not exists
  if [[ ! -f "${CONFIG_DIR}/config" ]]; then
    if [[ -f "${SCRIPT_DIR}/config/default.conf" ]]; then
      cp "${SCRIPT_DIR}/config/default.conf" "${CONFIG_DIR}/config"
      log "  Created: ${CONFIG_DIR}/config"
    else
      cat > "${CONFIG_DIR}/config" <<'EOF'
WIZADO_RESOLUTION=auto
WIZADO_FSR=off
WIZADO_FRAMELIMIT=0
WIZADO_VRR=off
WIZADO_MANGOHUD=off
WIZADO_STEAM_UI=tenfoot
WIZADO_WORKSPACE=10
EOF
      log "  Created: ${CONFIG_DIR}/config (default)"
    fi
  else
    log "  Config already exists: ${CONFIG_DIR}/config"
  fi

  # Ensure ~/.local/bin is in PATH
  if [[ ":$PATH:" != *":${LOCAL_BIN}:"* ]]; then
    echo ""
    warn "${LOCAL_BIN} is not in your PATH"
    warn "Add to ~/.bashrc: export PATH=\"\$PATH:${LOCAL_BIN}\""
  fi
}

# ============================================================================
# Hyprland Keybindings
# ============================================================================

configure_shortcuts() {
  log "Configuring Hyprland keybindings..."

  # Remove old bindings
  sed -i '/# Gaming Mode bindings - added by wizado/,/# End Gaming Mode bindings/d' "$BINDINGS_CONFIG" 2>/dev/null || true
  sed -i '/# Steam - added by wizado/,/# End Steam bindings/d' "$BINDINGS_CONFIG" 2>/dev/null || true
  sed -i '/# Wizado - added by wizado/,/# End Wizado bindings/d' "$BINDINGS_CONFIG" 2>/dev/null || true

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
    echo "# Wizado - added by wizado"
    echo "# Steam gaming launcher (runs on workspace, Ctrl+Alt+arrows to switch)"
    if [[ "$bind_style" == "bindd" ]]; then
      echo "bindd = SUPER SHIFT, S, Steam, exec, ${LOCAL_BIN}/wizado"
      echo "bindd = SUPER SHIFT, Q, Kill Steam, exec, pkill -9 steam; pkill -9 gamescope"
    else
      echo "bind = SUPER SHIFT, S, exec, ${LOCAL_BIN}/wizado"
      echo "bind = SUPER SHIFT, Q, exec, pkill -9 steam; pkill -9 gamescope"
    fi
    echo "# End Wizado bindings"
  } >> "$BINDINGS_CONFIG" || die "Failed to add keybinding"

  ADDED_BINDINGS=1

  hyprctl reload >/dev/null 2>&1 || warn "Hyprland reload may have failed"
  log "Keybindings added to $BINDINGS_CONFIG"
}

# ============================================================================
# Main
# ============================================================================

usage() {
  cat <<'EOF'
wizado setup

Installs Steam gaming launcher for Hyprland.

Options:
  --yes, -y     Non-interactive mode
  --dry-run     Print what would be done
  -h, --help    Show this help

What this does:
  • Installs Steam, gamescope, and GPU drivers
  • Installs wizado launcher to ~/.local/bin
  • Creates config at ~/.config/wizado/config
  • Adds Hyprland keybindings

Keybindings after install:
  Super + Shift + S    Launch Steam (gamescope on workspace 10)
  Super + Shift + Q    Force-quit Steam

Edit settings: wizado-config
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
  echo "  WIZADO - Steam Gaming Launcher Setup"
  echo "════════════════════════════════════════════════════════════════"
  echo ""

  validate_environment
  log "Using bindings config: $BINDINGS_CONFIG"

  confirm_or_die "Install wizado Steam gaming launcher?"

  detect_all_gpus
  ensure_multilib_enabled
  check_dependencies
  install_optional_packages
  check_user_groups
  maybe_grant_gamescope_cap
  deploy_scripts
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
  echo "  Keybindings:"
  echo "    Super + Shift + S    Launch Steam"
  echo "    Super + Shift + Q    Force-quit Steam"
  echo ""
  echo "  Commands:"
  echo "    wizado               Launch Steam"
  echo "    wizado-config        Configure settings"
  echo ""
  echo "  Config: ${CONFIG_DIR}/config"
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

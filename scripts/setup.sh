#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"

TARGET_DIR="${HOME}/.local/share/steam-launcher"
SWITCH_BIN="${TARGET_DIR}/enter-gamesmode"
RETURN_BIN="${TARGET_DIR}/leave-gamesmode"

HYPR_DIR="${HOME}/.config/hypr"
HYPR_MAIN_CONFIG="${HYPR_DIR}/hyprland.conf"
HYPR_INCLUDE_DIR="${HYPR_DIR}/conf.d"
HYPR_INCLUDE_FILE="${HYPR_INCLUDE_DIR}/the-wizard.conf"
HYPR_SOURCE_LINE="source = ${HYPR_INCLUDE_FILE} # the-wizard:source"

ENV_DIR="${HOME}/.config/environment.d"
INTEL_ARC_ENV_FILE="${ENV_DIR}/10-intel-arc-gtk.conf"

SUDOERS_DIR="/etc/sudoers.d"
GAMESMODE_SUDOERS_FILE="${SUDOERS_DIR}/the-wizard-gamesmode"

WAYBAR_DIR="${HOME}/.config/waybar"
WAYBAR_CONFIG_JSONC="${WAYBAR_DIR}/config.jsonc"
WAYBAR_SCRIPTS_DIR="${WAYBAR_DIR}/scripts"
WAYBAR_WIZARD_STATUS_SCRIPT="${WAYBAR_SCRIPTS_DIR}/the-wizard-status.sh"

INSTALLER_STARTED=0

usage() {
  cat <<'EOF'
Usage: ./scripts/setup.sh [--yes|-y] [--dry-run]

Installs Steam + required dependencies for Omarchy 3.2 (Arch/Hyprland/Wayland),
and configures a Hyprland keybind to launch Steam in a gamescope session.

Options:
  --yes, -y     Non-interactive; assume "yes" for confirmations.
  --dry-run     Print commands without running them.
EOF
}

die() {
  # Override common.sh die() to allow rollback for this installer.
  local msg="${1:-unknown error}"
  warn "$msg"
  if [[ "${INSTALLER_STARTED:-0}" -eq 1 ]]; then
    rollback_changes || true
  fi
  printf "[%s] ERROR: %s\n" "$PROJECT_NAME" "$msg" >&2
  exit 1
}

run_sudo() {
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: sudo $*"
    return 0
  fi
  log "RUN: sudo $*"
  sudo "$@"
}

check_package_installed() {
  pacman -Qi "$1" >/dev/null 2>&1
}

install_pacman_packages() {
  local -a pkgs=("$@")
  local -a missing=()
  local p
  for p in "${pkgs[@]}"; do
    check_package_installed "$p" || missing+=("$p")
  done
  if ((${#missing[@]} == 0)); then
    log "All required packages already installed."
    return 0
  fi

  log "Installing packages: ${missing[*]}"
  run_sudo pacman -S --needed ${ASSUME_YES:+--noconfirm} "${missing[@]}" || die "Failed to install packages via pacman"

  for p in "${missing[@]}"; do
    record_installed_item "pacman:${p}"
  done
}

detect_gpu_vendor() {
  # Outputs: nvidia|amd|intel|unknown
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
  # Returns: arc|igpu|both|none
  local has_arc=false
  local has_igpu=false

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

ensure_multilib_enabled() {
  if grep -q "^[[:space:]]*\\[multilib\\]" /etc/pacman.conf 2>/dev/null; then
    log "Multilib repository: enabled"
    return 0
  fi

  warn "Multilib repository is NOT enabled, but is required for Steam (32-bit libraries)."
  confirm_or_die "Enable multilib in /etc/pacman.conf now?"

  local backup="/etc/pacman.conf.backup.the-wizard.$(date +%Y%m%d%H%M%S)"
  run_sudo cp /etc/pacman.conf "$backup" || die "Failed to backup /etc/pacman.conf"
  log "Backed up /etc/pacman.conf to: $backup"

  # Uncomment the multilib block if present.
  run_sudo sed -i '/^#\\[multilib\\]/,/^#Include/ s/^#//' /etc/pacman.conf || die "Failed to enable multilib"

  if ! grep -q "^[[:space:]]*\\[multilib\\]" /etc/pacman.conf 2>/dev/null; then
    die "Multilib enablement did not succeed; please enable it manually and re-run."
  fi

  log "Refreshing package DB after enabling multilib"
  run_sudo pacman -Syy ${ASSUME_YES:+--noconfirm} || die "Failed to refresh package database"
}

setup_intel_arc_gtk_workaround() {
  local intel_type
  intel_type="$(detect_intel_gpu_type)"
  [[ "$intel_type" == "arc" || "$intel_type" == "both" ]] || return 0

  mkdir -p "$ENV_DIR" || die "Failed to create: $ENV_DIR"

  if [[ -f "$INTEL_ARC_ENV_FILE" ]] && grep -q "^GSK_RENDERER=" "$INTEL_ARC_ENV_FILE" 2>/dev/null; then
    log "Intel Arc GTK workaround already present: $INTEL_ARC_ENV_FILE"
    return 0
  fi

  log "Configuring Intel Arc GTK4 workaround (GSK_RENDERER=gl)"
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: write $INTEL_ARC_ENV_FILE"
  else
    cat >"$INTEL_ARC_ENV_FILE" <<'EOF'
# Intel Arc GTK4 Rendering Fix
# Added by the-wizard
#
# Fixes visual glitches in some GTK4 apps on Wayland with Intel Arc GPUs.
GSK_RENDERER=gl
EOF
  fi

  record_installed_item "file:${INTEL_ARC_ENV_FILE}"
  warn "Intel Arc GTK fix installed. You may need to log out and log back in."
}

maybe_grant_gamescope_cap() {
  require_cmd getcap
  require_cmd setcap

  local gamescope_path
  gamescope_path="$(command -v gamescope 2>/dev/null || true)"
  [[ -n "$gamescope_path" ]] || return 0

  if getcap "$gamescope_path" 2>/dev/null | grep -q 'cap_sys_nice'; then
    log "gamescope already has cap_sys_nice"
    return 0
  fi

  warn "Optional: grant cap_sys_nice to gamescope to allow --rt (lower latency)."
  confirm_or_die "Grant cap_sys_nice to gamescope?"

  run_sudo setcap 'cap_sys_nice+ep' "$gamescope_path" || die "Failed to setcap on gamescope"
  record_installed_item "setcap:${gamescope_path}"
}

detect_bindings_config_file() {
  local f
  for f in \
    "${HYPR_DIR}/bindings.conf" \
    "${HYPR_DIR}/keybinds.conf" \
    "${HYPR_MAIN_CONFIG}"; do
    if [[ -f "$f" ]]; then
      echo "$f"
      return 0
    fi
  done
  echo ""
}

write_launchers() {
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: create ${TARGET_DIR} and launchers"
  else
    mkdir -p "$TARGET_DIR" || die "Failed to create: $TARGET_DIR"
  fi
  record_installed_item "file:${TARGET_DIR}"

  local launcher_src_dir
  launcher_src_dir="${SCRIPT_DIR}/launchers"

  if [[ ! -f "${launcher_src_dir}/enter-gamesmode" || ! -f "${launcher_src_dir}/leave-gamesmode" ]]; then
    die "Launcher templates not found under: ${launcher_src_dir}"
  fi

  # enter-gamesmode
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: install ${launcher_src_dir}/enter-gamesmode -> ${SWITCH_BIN}"
  else
    install -Dm755 "${launcher_src_dir}/enter-gamesmode" "$SWITCH_BIN" || die "Failed to install: $SWITCH_BIN"
  fi
  record_installed_item "file:${SWITCH_BIN}"

  # leave-gamesmode
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: install ${launcher_src_dir}/leave-gamesmode -> ${RETURN_BIN}"
  else
    install -Dm755 "${launcher_src_dir}/leave-gamesmode" "$RETURN_BIN" || die "Failed to install: $RETURN_BIN"
  fi
  record_installed_item "file:${RETURN_BIN}"

  # Wizard intentionally removed (user requested couch-mode only).
}

maybe_install_gamesmode_sudoers() {
  # Needed for optional GAMESMODE_MODE=tty (openvt/chvt) so Hypr hotkey won't hang on a password prompt.
  [[ -d "$SUDOERS_DIR" ]] || return 0

  warn "Optional: enable passwordless sudo for /usr/bin/openvt and /usr/bin/chvt (required for exclusive TTY gamescope mode)."
  confirm_or_die "Install sudoers drop-in at ${GAMESMODE_SUDOERS_FILE}?"

  local username
  username="$(id -un)"

  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: write sudoers ${GAMESMODE_SUDOERS_FILE}"
    return 0
  fi

  run_sudo bash -lc "cat >'${GAMESMODE_SUDOERS_FILE}' <<'EOF'
# Added by the-wizard for optional exclusive TTY gamescope mode.
# Allows Hyprland hotkeys to switch VTs without a password prompt.
${username} ALL=(root) NOPASSWD: /usr/bin/openvt, /usr/bin/chvt
EOF
chmod 0440 '${GAMESMODE_SUDOERS_FILE}'
visudo -cf '${GAMESMODE_SUDOERS_FILE}'
"

  record_installed_item "file:${GAMESMODE_SUDOERS_FILE}"
}

write_hypr_include() {
  mkdir -p "$HYPR_INCLUDE_DIR" || die "Failed to create: $HYPR_INCLUDE_DIR"

  local bindings_file
  bindings_file="$(detect_bindings_config_file)"
  if [[ -z "$bindings_file" ]]; then
    die "Could not find Hypr config (expected under: ${HYPR_DIR})"
  fi

  # Detect bind style based on existing config.
  local bind_style="bindd"
  if ! grep -q "^bindd[[:space:]]*=" "$bindings_file" 2>/dev/null; then
    if grep -q "^bind[[:space:]]*=" "$bindings_file" 2>/dev/null; then
      bind_style="bind"
    fi
  fi

  log "Writing Hypr include: $HYPR_INCLUDE_FILE (bind style: $bind_style)"
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: write $HYPR_INCLUDE_FILE"
  else
    cat >"$HYPR_INCLUDE_FILE" <<EOF
# the-wizard Hyprland bindings (Omarchy 3.2)
# Generated by the-wizard setup.sh
EOF

    if [[ "$bind_style" == "bindd" ]]; then
      cat >>"$HYPR_INCLUDE_FILE" <<EOF
bindd = SUPER SHIFT, S, Steam (normal), exec, $SWITCH_BIN --mode nested
bindd = SUPER ALT, S, Steam (wizard), exec, $SWITCH_BIN --mode tty
bindd = SUPER SHIFT, R, Exit Couch Mode, exec, $RETURN_BIN
EOF
    else
      cat >>"$HYPR_INCLUDE_FILE" <<EOF
bind = SUPER SHIFT, S, exec, $SWITCH_BIN --mode nested
bind = SUPER ALT, S, exec, $SWITCH_BIN --mode tty
bind = SUPER SHIFT, R, exec, $RETURN_BIN
EOF
    fi
  fi
  record_installed_item "file:${HYPR_INCLUDE_FILE}"
}

maybe_install_waybar_module() {
  [[ -d "$WAYBAR_DIR" ]] || return 0
  mkdir -p "$WAYBAR_SCRIPTS_DIR" || die "Failed to create: $WAYBAR_SCRIPTS_DIR"

  warn "Optional: install a Waybar icon for the-wizard (left-click normal, right-click wizard, middle-click exit)."
  confirm_or_die "Install Waybar script at ${WAYBAR_WIZARD_STATUS_SCRIPT} (and patch config.jsonc if present)?"

  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: install waybar script"
  else
    install -Dm755 "${SCRIPT_DIR}/waybar/the-wizard-status.sh" "$WAYBAR_WIZARD_STATUS_SCRIPT" || die "Failed to install: $WAYBAR_WIZARD_STATUS_SCRIPT"
  fi
  record_installed_item "file:${WAYBAR_WIZARD_STATUS_SCRIPT}"

  [[ -f "$WAYBAR_CONFIG_JSONC" ]] || return 0

  # Patch config.jsonc in an idempotent way (with backup).
  if grep -q '"custom/the-wizard"' "$WAYBAR_CONFIG_JSONC" 2>/dev/null; then
    log "Waybar already configured for custom/the-wizard"
    return 0
  fi

  warn "Patching Waybar config: ${WAYBAR_CONFIG_JSONC} (will create a backup)."
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: patch waybar config"
    return 0
  fi

  cp -a "$WAYBAR_CONFIG_JSONC" "${WAYBAR_CONFIG_JSONC}.bak.the-wizard.$(date +%Y%m%d%H%M%S)" || die "Failed to backup waybar config"

  # 1) Add module to modules-left after custom/omarchy if present, else append.
  python3 - <<'PY'
import json, re, pathlib, sys

path = pathlib.Path.home() / ".config/waybar/config.jsonc"
raw = path.read_text(encoding="utf-8")

# Very small jsonc stripper (removes // comments). Omarchy config.jsonc is plain JSON today.
raw2 = re.sub(r"//.*?$", "", raw, flags=re.M)
cfg = json.loads(raw2)

def inject_module(arr):
    if "custom/the-wizard" in arr:
        return arr
    if "custom/omarchy" in arr:
        i = arr.index("custom/omarchy") + 1
        return arr[:i] + ["custom/the-wizard"] + arr[i:]
    return arr + ["custom/the-wizard"]

cfg["modules-left"] = inject_module(cfg.get("modules-left", []))

cfg["custom/the-wizard"] = {
    "format": "{icon}",
    "return-type": "json",
    "exec": str(pathlib.Path.home() / ".config/waybar/scripts/the-wizard-status.sh"),
    "interval": 2,
    "on-click": str(pathlib.Path.home() / ".local/share/steam-launcher/enter-gamesmode") + " --mode nested",
    "on-click-right": str(pathlib.Path.home() / ".local/share/steam-launcher/enter-gamesmode") + " --mode tty",
    "on-click-middle": str(pathlib.Path.home() / ".local/share/steam-launcher/leave-gamesmode"),
    "tooltip": True
}

out = json.dumps(cfg, indent=2)
path.write_text(out + "\n", encoding="utf-8")
PY
}

ensure_hypr_source_line() {
  if [[ ! -f "$HYPR_MAIN_CONFIG" ]]; then
    die "Hyprland main config not found: $HYPR_MAIN_CONFIG"
  fi

  if grep -Fq "$HYPR_SOURCE_LINE" "$HYPR_MAIN_CONFIG" 2>/dev/null; then
    log "Hypr source line already present."
    return 0
  fi

  log "Adding Hypr source line to: $HYPR_MAIN_CONFIG"
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: append source line"
  else
    printf "\n%s\n" "$HYPR_SOURCE_LINE" >>"$HYPR_MAIN_CONFIG" || die "Failed to update Hypr config"
  fi
  record_installed_item "hyprsource:${HYPR_MAIN_CONFIG}"
}

rollback_changes() {
  # Best-effort rollback for files we created during this run.
  # Full removal is handled by scripts/remove.sh using .state.
  [[ -f "$SWITCH_BIN" ]] && rm -f "$SWITCH_BIN" || true
  [[ -f "$RETURN_BIN" ]] && rm -f "$RETURN_BIN" || true
  [[ -f "$HYPR_INCLUDE_FILE" ]] && rm -f "$HYPR_INCLUDE_FILE" || true
}

main() {
  if ! parse_common_flags "$@"; then
    usage
    exit 0
  fi

require_cmd pacman
require_cmd hyprctl

confirm_or_die "This will install/configure Steam and Hyprland bindings. Continue?"
INSTALLER_STARTED=1

ensure_state_dir
clear_installed_items

if [[ ! -d "$HYPR_DIR" ]]; then
  die "Hyprland config dir not found: $HYPR_DIR"
fi

ensure_multilib_enabled

install_pacman_packages pciutils

local -a required_pkgs=(
  steam
  xdg-user-dirs
  mesa
  lib32-mesa
  vulkan-icd-loader
  lib32-vulkan-icd-loader
)

local gpu_vendor
gpu_vendor="$(detect_gpu_vendor)"
log "Detected GPU vendor: $gpu_vendor"

case "$gpu_vendor" in
  nvidia)
    required_pkgs+=(nvidia-utils lib32-nvidia-utils)
    ;;
  amd)
    required_pkgs+=(vulkan-radeon lib32-vulkan-radeon)
    ;;
  intel)
    required_pkgs+=(vulkan-intel lib32-vulkan-intel)
    ;;
  *)
    warn "GPU vendor not detected. Installing both vulkan-intel and vulkan-radeon for coverage."
    required_pkgs+=(vulkan-intel lib32-vulkan-intel vulkan-radeon lib32-vulkan-radeon)
    ;;
esac

install_pacman_packages "${required_pkgs[@]}"

  local -a recommended_pkgs=(gamescope gamemode mangohud libcap vulkan-tools kbd)
warn "Recommended packages: ${recommended_pkgs[*]}"
confirm_or_die "Install recommended packages?"
install_pacman_packages "${recommended_pkgs[@]}"

setup_intel_arc_gtk_workaround

maybe_grant_gamescope_cap
  maybe_install_gamesmode_sudoers

write_launchers
write_hypr_include
ensure_hypr_source_line
  maybe_install_waybar_module

log "Reloading Hyprland config"
run hyprctl reload || warn "hyprctl reload failed; you may need to reload/relog manually."

log "Install complete"
log "Launch: Super+Shift+S"
log "Exit:   Super+Shift+R"
log "Binaries: $TARGET_DIR"
log "Hypr include: $HYPR_INCLUDE_FILE"
}

main "$@"



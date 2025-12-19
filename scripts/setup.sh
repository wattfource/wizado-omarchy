#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"
# shellcheck source=lib/license.sh
source "$SCRIPT_DIR/lib/license.sh"

TARGET_DIR="${HOME}/.local/share/steam-launcher"
SWITCH_BIN="${TARGET_DIR}/enter-gamesmode"
RETURN_BIN="${TARGET_DIR}/leave-gamesmode"
MENU_BIN="${TARGET_DIR}/wizado-menu"

HYPR_DIR="${HOME}/.config/hypr"
HYPR_MAIN_CONFIG="${HYPR_DIR}/hyprland.conf"
HYPR_INCLUDE_DIR="${HYPR_DIR}/conf.d"
HYPR_INCLUDE_FILE="${HYPR_INCLUDE_DIR}/wizado.conf"
HYPR_SOURCE_LINE="source = ${HYPR_INCLUDE_FILE} # wizado:source"

ENV_DIR="${HOME}/.config/environment.d"
INTEL_ARC_ENV_FILE="${ENV_DIR}/10-intel-arc-gtk.conf"

SUDOERS_DIR="/etc/sudoers.d"
GAMESMODE_SUDOERS_FILE="${SUDOERS_DIR}/wizado-gamesmode"

WAYBAR_DIR="${HOME}/.config/waybar"
WAYBAR_CONFIG_JSONC="${WAYBAR_DIR}/config.jsonc"
WAYBAR_SCRIPTS_DIR="${WAYBAR_DIR}/scripts"
WAYBAR_WIZARD_STATUS_SCRIPT="${WAYBAR_SCRIPTS_DIR}/wizado-status.sh"

INSTALLER_STARTED=0

maybe_install_updated_cli_shim() {
  # If the system-installed wizado is older (missing newer subcommands like enable-tty),
  # offer to install an updated CLI shim into /usr/local/bin/wizado (typically precedes /usr/bin in PATH).
  #
  # This is Omarchy-focused: users commonly run setup from a git checkout before reinstalling the package.
  local root_dir
  root_dir="$(cd "$SCRIPT_DIR/.." && pwd)"
  local new_cli="${root_dir}/bin/wizado"

  [[ -f "$new_cli" ]] || return 0
  [[ -x "$new_cli" ]] || return 0

  if ! command -v wizado >/dev/null 2>&1; then
    return 0
  fi

  if wizado --help 2>/dev/null | grep -q "enable-tty"; then
    return 0
  fi

  warn "Your current wizado CLI ($(command -v wizado)) is older and does not include 'enable-tty'."
  warn "Optional: install updated CLI to /usr/local/bin/wizado so you can run: wizado enable-tty"
  confirm_or_die "Install updated wizado CLI to /usr/local/bin/wizado?"

  run_sudo install -Dm755 "$new_cli" /usr/local/bin/wizado || die "Failed to install /usr/local/bin/wizado"
  record_installed_item "file:/usr/local/bin/wizado"
  log "Installed updated CLI: /usr/local/bin/wizado"
}

cleanup_wizado_processes() {
  # Best-effort cleanup of wizado-launched sessions so installs/updates don't race a running gamescope/Steam.
  #
  # Safety goals:
  # - Prefer graceful Steam shutdown
  # - Only kill gamescope processes that appear to be running Steam (wizado couch-mode),
  #   rather than killing arbitrary gamescope sessions the user might be using for other apps.
  local state_dir="${HOME}/.cache/wizado"
  local session_file="${state_dir}/session.pid"

  log "Pre-flight: cleaning up any running wizado session (best-effort)"

  # If the installed leave-gamesmode exists, use it (it handles Steam shutdown + PID cleanup).
  if [[ -x "${RETURN_BIN}" ]]; then
    "${RETURN_BIN}" >/dev/null 2>&1 || true
  fi

  # Graceful Steam shutdown (safe even if Steam isn't running).
  if command -v steam >/dev/null 2>&1; then
    steam -shutdown >/dev/null 2>&1 || true
    # Give it a moment to exit cleanly.
    for _ in 1 2 3 4 5 6 7 8 9 10; do
      pgrep -x steam >/dev/null 2>&1 || break
      sleep 0.3
    done
  fi

  # If wizado recorded a session PID, try to terminate it first.
  if [[ -f "$session_file" ]]; then
    local pid=""
    pid="$(cat "$session_file" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      sleep 0.3
      kill -0 "$pid" >/dev/null 2>&1 && kill -9 "$pid" >/dev/null 2>&1 || true
    fi
  fi

  # Clean up any lingering Steam processes (shouldn't "mess up the OS"; worst case Steam relaunches).
  pkill -x steam >/dev/null 2>&1 || true
  pkill -x steamwebhelper >/dev/null 2>&1 || true

  # Kill only gamescope instances that look like they're running Steam (wizado couch-mode).
  # pgrep -af prints: "<pid> <cmdline>"
  if command -v pgrep >/dev/null 2>&1; then
    while IFS= read -r line; do
      [[ -n "$line" ]] || continue
      pid="${line%% *}"
      cmd="${line#* }"
      # Match wizado-style gamescope invocation: "... gamescope ... -- steam -gamepadui" / "-tenfoot"
      if echo "$cmd" | grep -Eq -- '(^|[[:space:]])gamescope([[:space:]].*)?[[:space:]]--[[:space:]]steam([[:space:]].*)?(-gamepadui|-tenfoot)'; then
        kill "$pid" >/dev/null 2>&1 || true
        sleep 0.2
        kill -0 "$pid" >/dev/null 2>&1 && kill -9 "$pid" >/dev/null 2>&1 || true
      fi
    done < <(pgrep -af gamescope 2>/dev/null || true)
  fi

  # If enter-gamesmode itself is still running (e.g., stuck), stop it.
  pkill -f "${HOME}/.local/share/steam-launcher/enter-gamesmode" >/dev/null 2>&1 || true

  # Don't delete state; let leave-gamesmode manage cleanup. Just ensure we don't keep stale pid around.
  rm -f "$session_file" >/dev/null 2>&1 || true
}

usage() {
  cat <<'EOF'
Usage: ./scripts/setup.sh [--yes|-y] [--dry-run]

Omarchy-only installer (Arch/Hyprland/Wayland).

Installs Steam + required dependencies and configures Omarchy/Hyprland bindings
to launch Steam in a gamescope session.

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

  local backup="/etc/pacman.conf.backup.wizado.$(date +%Y%m%d%H%M%S)"
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
  else
    log "Configuring Intel Arc GTK4 workaround (GSK_RENDERER=gl)"
    if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
      log "DRY-RUN: write $INTEL_ARC_ENV_FILE"
    else
      cat >"$INTEL_ARC_ENV_FILE" <<'EOF'
# Intel Arc GTK4 Rendering Fix
# Added by wizado
#
# Fixes visual glitches in some GTK4 apps on Wayland with Intel Arc GPUs.
GSK_RENDERER=gl
EOF
    fi
  fi
  # Always record it so we can remove it later
  record_installed_item "file:${INTEL_ARC_ENV_FILE}"
  warn "Intel Arc GTK fix installed. You may need to log out and log back in."
}

check_user_groups() {
  local missing_groups=()
  
  if ! groups | grep -q '\bvideo\b'; then
    missing_groups+=("video")
  fi
  
  if ! groups | grep -q '\brender\b'; then
    missing_groups+=("render")
  fi
  
  if ! groups | grep -q '\binput\b'; then
    missing_groups+=("input")
  fi
  
  if ((${#missing_groups[@]} == 0)); then
    log "User groups (video, render, input): OK"
    return 0
  fi
  
  warn "User '$USER' is missing from groups: ${missing_groups[*]}"
  warn "These are required for GPU access and controller support."
  confirm_or_die "Add user '$USER' to groups ${missing_groups[*]}?"
  
  local groups_csv
  groups_csv=$(IFS=,; echo "${missing_groups[*]}")
  
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: sudo usermod -aG $groups_csv $USER"
  else
    run_sudo usermod -aG "$groups_csv" "$USER" || die "Failed to add user to groups"
    warn "User added to groups. You MUST log out and log back in for this to take effect."
  fi
}

setup_performance_rules() {
  local rule_file="/etc/udev/rules.d/99-gaming-performance.rules"
  
  if [[ -f "$rule_file" ]]; then
    log "Performance udev rules already present."
    return 0
  fi
  
  warn "Optional: install udev rules for passwordless CPU/GPU performance control."
  confirm_or_die "Install 99-gaming-performance.rules?"
  
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: write $rule_file"
    return 0
  fi
  
  # Note: logic adapted from original install script
  run_sudo bash -c "cat >'$rule_file' <<'EOF'
# Gaming Mode Performance Control Rules
# Allow users to modify CPU governor and GPU performance settings without sudo

# CPU governor control (all CPUs)
KERNEL==\"cpu[0-9]*\", SUBSYSTEM==\"cpu\", ACTION==\"add\", RUN+=\"/bin/chmod 666 /sys/devices/system/cpu/%k/cpufreq/scaling_governor\"

# AMD GPU performance control
KERNEL==\"card[0-9]\", SUBSYSTEM==\"drm\", DRIVERS==\"amdgpu\", ACTION==\"add\", RUN+=\"/bin/chmod 666 /sys/class/drm/%k/device/power_dpm_force_performance_level\"

# Intel GPU frequency control (i915 driver)
KERNEL==\"card[0-9]\", SUBSYSTEM==\"drm\", DRIVERS==\"i915\", ACTION==\"add\", RUN+=\"/bin/chmod 666 /sys/class/drm/%k/gt_boost_freq_mhz\"
KERNEL==\"card[0-9]\", SUBSYSTEM==\"drm\", DRIVERS==\"i915\", ACTION==\"add\", RUN+=\"/bin/chmod 666 /sys/class/drm/%k/gt_min_freq_mhz\"
KERNEL==\"card[0-9]\", SUBSYSTEM==\"drm\", DRIVERS==\"i915\", ACTION==\"add\", RUN+=\"/bin/chmod 666 /sys/class/drm/%k/gt_max_freq_mhz\"
EOF"

  log "Reloading udev rules"
  run_sudo udevadm control --reload-rules
  run_sudo udevadm trigger --subsystem-match=cpu --subsystem-match=drm
  
  record_installed_item "file:$rule_file"
  warn "Performance rules installed."
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

  # wizado-menu
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: install ${launcher_src_dir}/wizado-menu -> ${MENU_BIN}"
  else
    install -Dm755 "${launcher_src_dir}/wizado-menu" "$MENU_BIN" || die "Failed to install: $MENU_BIN"
  fi
  record_installed_item "file:${MENU_BIN}"

  # Wizard intentionally removed (user requested couch-mode only).
}

maybe_install_gamesmode_sudoers() {
  # Needed for optional GAMESMODE_MODE=tty (openvt/chvt) so Hypr hotkey won't hang on a password prompt.
  [[ -d "$SUDOERS_DIR" ]] || return 0

  warn "Optional (but REQUIRED for Super+Alt+S / TTY mode): enable passwordless sudo for /usr/bin/openvt and /usr/bin/chvt."
  warn "Without this, TTY mode cannot switch VTs from a Hyprland hotkey."
  confirm_or_die "Install sudoers drop-in at ${GAMESMODE_SUDOERS_FILE}?"

  local username
  username="$(id -un)"

  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: write sudoers ${GAMESMODE_SUDOERS_FILE}"
    return 0
  fi

  run_sudo bash -lc "cat >'${GAMESMODE_SUDOERS_FILE}' <<'EOF'
# Added by wizado for optional exclusive TTY gamescope mode.
# Allows Hyprland hotkeys to switch VTs without a password prompt.
${username} ALL=(root) NOPASSWD: /usr/bin/openvt, /usr/bin/chvt
EOF
chmod 0440 '${GAMESMODE_SUDOERS_FILE}'
visudo -cf '${GAMESMODE_SUDOERS_FILE}'
"

  record_installed_item "file:${GAMESMODE_SUDOERS_FILE}"
}

detect_terminal() {
  # Returns: terminal_cmd, or empty if none found
  local terms=("ghostty" "alacritty" "kitty" "foot" "gnome-terminal" "konsole")
  for t in "${terms[@]}"; do
    if command -v "$t" >/dev/null 2>&1; then
      if [[ "$t" == "gnome-terminal" ]]; then
        echo "$t --"
      elif [[ "$t" == "konsole" ]]; then
        echo "$t -e"
      else
        echo "$t -e" # alacritty, kitty, foot, ghostty all support -e
      fi
      return 0
    fi
  done
  echo ""
}

write_hypr_include() {
  mkdir -p "$HYPR_INCLUDE_DIR" || die "Failed to create: $HYPR_INCLUDE_DIR"

  local bindings_file
  bindings_file="$(detect_bindings_config_file)"
  if [[ -z "$bindings_file" ]]; then
    die "Could not find Hypr config (expected under: ${HYPR_DIR})"
  fi

  local term_cmd
  term_cmd="$(detect_terminal)"
  if [[ -z "$term_cmd" ]]; then
     term_cmd="alacritty -e" # Fallback
     warn "No supported terminal found. Defaulting to: $term_cmd"
  fi

  # Detect bind style based on existing config.
  local bind_style="bindd"
  if ! grep -q "^bindd[[:space:]]*=" "$bindings_file" 2>/dev/null; then
    if grep -q "^bind[[:space:]]*=" "$bindings_file" 2>/dev/null; then
      bind_style="bind"
    fi
  fi

  if [[ -f "$HYPR_INCLUDE_FILE" ]]; then
    log "Hypr include already exists: $HYPR_INCLUDE_FILE"
    log "Skipping overwrite to preserve user keybindings."
    record_installed_item "file:${HYPR_INCLUDE_FILE}"
    return 0
  fi

  log "Writing Hypr include: $HYPR_INCLUDE_FILE (bind style: $bind_style)"
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: write $HYPR_INCLUDE_FILE"
  else
    cat >"$HYPR_INCLUDE_FILE" <<EOF
# wizado Hyprland bindings (Omarchy 3.2)
# Generated by wizado setup.sh
EOF

    if [[ "$bind_style" == "bindd" ]]; then
      cat >>"$HYPR_INCLUDE_FILE" <<EOF
unbind = SUPER ALT, S
bindd = SUPER SHIFT, S, Steam (nested), exec, $SWITCH_BIN --mode nested --steam-ui normal
bindd = SUPER ALT, S, Steam (performance), exec, $SWITCH_BIN --mode performance --steam-ui bigpicture
bindd = SUPER SHIFT, R, Exit Couch Mode, exec, $RETURN_BIN
EOF
    else
      cat >>"$HYPR_INCLUDE_FILE" <<EOF
unbind = SUPER ALT, S
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

  warn "Optional: install a Waybar icon for wizado (left-click to open menu)."
  confirm_or_die "Install Waybar script at ${WAYBAR_WIZARD_STATUS_SCRIPT} (and patch config.jsonc if present)?"

  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: install waybar script"
  else
    install -Dm755 "${SCRIPT_DIR}/waybar/wizado-status.sh" "$WAYBAR_WIZARD_STATUS_SCRIPT" || die "Failed to install: $WAYBAR_WIZARD_STATUS_SCRIPT"
  fi
  record_installed_item "file:${WAYBAR_WIZARD_STATUS_SCRIPT}"

  [[ -f "$WAYBAR_CONFIG_JSONC" ]] || return 0

  # Patch config.jsonc in an idempotent way (with backup).
  # Note: even if custom/wizado already exists, we may need to fix its placement
  # (e.g., moving it into a collapsed group drawer like group/tray-expander).

  warn "Patching Waybar config: ${WAYBAR_CONFIG_JSONC} (will create a backup)."
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: patch waybar config"
    return 0
  fi

  cp -a "$WAYBAR_CONFIG_JSONC" "${WAYBAR_CONFIG_JSONC}.bak.wizado.$(date +%Y%m%d%H%M%S)" || die "Failed to backup waybar config"

  # 1) Prefer adding custom/wizado inside an existing collapsed group drawer (Omarchy uses group/tray-expander).
  #    Fallback: add to modules-right near other status icons.
  python3 - <<PY
import json, re, pathlib, sys, shutil

path = pathlib.Path.home() / ".config/waybar/config.jsonc"

def get_terminal_cmd():
    # Helper to find a terminal command (basic version of bash logic)
    terms = ["ghostty", "alacritty", "kitty", "foot", "gnome-terminal", "konsole"]
    for t in terms:
        if shutil.which(t):
            if t == "gnome-terminal": return t + " --"
            if t == "konsole": return t + " -e"
            return t + " -e"
    return "alacritty -e"

try:
    raw = path.read_text(encoding="utf-8")
    # Very small jsonc stripper (removes // comments). Omarchy config.jsonc is plain JSON today.
    raw2 = re.sub(r"//.*?$", "", raw, flags=re.M)
    cfg = json.loads(raw2)

    def remove_module(arr, mod):
        if not isinstance(arr, list):
            return arr
        return [x for x in arr if x != mod]

    def inject_right(arr):
        if not isinstance(arr, list):
            return ["custom/wizado"]
        if "custom/wizado" in arr:
            return arr
        # Place before bluetooth or network if possible, else append
        for target in ["bluetooth", "network", "pulseaudio", "cpu", "battery"]:
            if target in arr:
                i = arr.index(target)
                return arr[:i] + ["custom/wizado"] + arr[i:]
        return arr + ["custom/wizado"]

    def find_drawer_group_key(cfg_obj):
        # Prefer Omarchy's tray expander if present.
        if isinstance(cfg_obj.get("group/tray-expander"), dict) and isinstance(cfg_obj["group/tray-expander"].get("modules"), list):
            return "group/tray-expander"
        # Otherwise, look for any group/* with a modules list (drawer-like).
        for k, v in cfg_obj.items():
            if not isinstance(k, str) or not k.startswith("group/"):
                continue
            if isinstance(v, dict) and isinstance(v.get("modules"), list):
                return k
        return None

    group_key = find_drawer_group_key(cfg)
    if group_key:
        group = cfg.get(group_key, {})
        mods = group.get("modules", [])
        if isinstance(mods, list):
            if "custom/wizado" not in mods:
                # Keep the first module as the always-visible one (usually the expand icon).
                # Insert after tray if present, else append (but never at index 0).
                if "tray" in mods:
                    i = mods.index("tray") + 1
                    mods = mods[:i] + ["custom/wizado"] + mods[i:]
                else:
                    mods = mods + ["custom/wizado"]
                    if mods and mods[0] == "custom/wizado":
                        mods = ["custom/expand-icon"] + mods
            group["modules"] = mods
            cfg[group_key] = group
        # Ensure it's not duplicated in the top-level module lists.
        for list_key in ("modules-right", "modules-left", "modules-center"):
            cfg[list_key] = remove_module(cfg.get(list_key, []), "custom/wizado")
    else:
        cfg["modules-right"] = inject_right(cfg.get("modules-right", []))

    term_cmd = get_terminal_cmd()

    cfg["custom/wizado"] = {
        "format": "{}",
        "return-type": "json",
        "exec": str(pathlib.Path.home() / ".config/waybar/scripts/wizado-status.sh"),
        "interval": 2,
        "on-click": term_cmd + " " + str(pathlib.Path.home() / ".local/share/steam-launcher/wizado-menu"),
        "tooltip": True
    }

    out = json.dumps(cfg, indent=2)
    path.write_text(out + "\n", encoding="utf-8")
except Exception as e:
    print(f"Skipping Waybar patch due to error: {e}", file=sys.stderr)
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

  # check_license

require_omarchy

maybe_install_updated_cli_shim

require_cmd pacman
require_cmd hyprctl

confirm_or_die "This will install/configure Steam and Hyprland bindings. Continue?"
INSTALLER_STARTED=1

cleanup_wizado_processes

ensure_state_dir
clear_installed_items

if [[ ! -d "$HYPR_DIR" ]]; then
  die "Hyprland config dir not found: $HYPR_DIR"
fi

ensure_multilib_enabled

install_pacman_packages pciutils

local -a required_pkgs=(
  steam
  python
  xdg-user-dirs
  mesa
  lib32-mesa
  vulkan-icd-loader
  lib32-vulkan-icd-loader
  mesa-utils
  lib32-systemd
  lib32-glibc
  lib32-gcc-libs
  lib32-libx11
  lib32-libxss
  lib32-alsa-plugins
  lib32-libpulse
  lib32-openal
  lib32-nss
  lib32-libcups
  lib32-sdl2
  lib32-freetype2
  lib32-fontconfig
  ttf-liberation
)

local gpu_vendor
gpu_vendor="$(detect_gpu_vendor)"
log "Detected GPU vendor: $gpu_vendor"

case "$gpu_vendor" in
  nvidia)
    required_pkgs+=(
      nvidia-utils 
      lib32-nvidia-utils 
      nvidia-settings 
      libva-nvidia-driver
    )
    # Optional check for DKMS modules handled via warning/manual install if missing
    ;;
  amd)
    required_pkgs+=(
      vulkan-radeon 
      lib32-vulkan-radeon
      libva-mesa-driver
      lib32-libva-mesa-driver
      mesa-vdpau
      lib32-mesa-vdpau
    )
    ;;
  intel)
    local intel_type
    intel_type="$(detect_intel_gpu_type)"
    
    if [[ "$intel_type" == "arc" ]]; then
      # Intel Arc discrete
      required_pkgs+=(
        vulkan-intel
        lib32-vulkan-intel
        intel-compute-runtime
        intel-gpu-tools
      )
      # level-zero-loader is good if available, but might be optional
    else
      # Integrated or older
      required_pkgs+=(
        vulkan-intel 
        lib32-vulkan-intel
        intel-media-driver
        libva-intel-driver
        lib32-libva-intel-driver
        intel-compute-runtime
        intel-gpu-tools
      )
    fi
    ;;
  *)
    warn "GPU vendor not detected. Installing both vulkan-intel and vulkan-radeon for coverage."
    required_pkgs+=(vulkan-intel lib32-vulkan-intel vulkan-radeon lib32-vulkan-radeon)
    ;;
esac

# Common Vulkan layers
required_pkgs+=(vulkan-mesa-layers)

install_pacman_packages "${required_pkgs[@]}"

  local -a recommended_deps=(
    gamescope 
    gamemode 
    lib32-gamemode 
    mangohud 
    lib32-mangohud 
    libcap 
    vulkan-tools 
    kbd
    wine
  )
warn "Recommended packages: ${recommended_deps[*]}"
confirm_or_die "Install recommended packages?"
install_pacman_packages "${recommended_deps[@]}"

setup_intel_arc_gtk_workaround
check_user_groups
setup_performance_rules

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



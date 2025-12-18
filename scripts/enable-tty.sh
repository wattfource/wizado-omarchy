#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"

SUDOERS_DIR="/etc/sudoers.d"
GAMESMODE_SUDOERS_FILE="${SUDOERS_DIR}/wizado-gamesmode"

usage() {
  cat <<'EOF'
Usage: wizado enable-tty [--yes|-y] [--dry-run] [--limine-conf /boot/limine.conf]

Enables prerequisites for "exclusive" TTY couch mode (Super+Alt+S):
  1) Install sudoers drop-in for passwordless /usr/bin/openvt and /usr/bin/chvt
  2) Ensure kernel parameter nvidia-drm.modeset=1 is set (Limine cmdline)

Notes:
  - Omarchy-only: expects Limine + UKI with cmdline in /boot/limine.conf.
  - Requires sudo privileges (will prompt for your password unless already authorized).
  - You must reboot after adding nvidia-drm.modeset=1.
EOF
}

run_sudo() {
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: sudo $*"
    return 0
  fi
  log "RUN: sudo $*"
  sudo "$@"
}

ensure_sudoers_for_openvt() {
  [[ -d "$SUDOERS_DIR" ]] || die "Missing: $SUDOERS_DIR"
  require_cmd visudo
  require_cmd openvt
  require_cmd chvt

  local username
  username="$(id -un)"

  if [[ -f "$GAMESMODE_SUDOERS_FILE" ]]; then
    if grep -qE "NOPASSWD: /usr/bin/openvt, /usr/bin/chvt" "$GAMESMODE_SUDOERS_FILE" 2>/dev/null; then
      log "Sudoers drop-in already present: $GAMESMODE_SUDOERS_FILE"
      return 0
    fi
    local backup="/etc/sudoers.d/wizado-gamesmode.bak.$(date +%Y%m%d%H%M%S)"
    run_sudo cp -a "$GAMESMODE_SUDOERS_FILE" "$backup"
    log "Backed up existing sudoers file to: $backup"
  fi

  confirm_or_die "Install sudoers drop-in for passwordless openvt/chvt at ${GAMESMODE_SUDOERS_FILE}?"

  run_sudo bash -lc "cat >'${GAMESMODE_SUDOERS_FILE}' <<'EOF'
# Added by wizado for exclusive TTY gamescope mode (Super+Alt+S).
# Allows Hyprland hotkeys to switch VTs without a password prompt.
${username} ALL=(root) NOPASSWD: /usr/bin/openvt, /usr/bin/chvt
EOF
chmod 0440 '${GAMESMODE_SUDOERS_FILE}'
visudo -cf '${GAMESMODE_SUDOERS_FILE}'
"

  log "Installed sudoers drop-in: $GAMESMODE_SUDOERS_FILE"
}

kernel_has_modeset() {
  grep -q "nvidia-drm.modeset=1" /proc/cmdline 2>/dev/null
}

append_param_to_options_line() {
  # args: file_path param
  local f="$1"
  local param="$2"

  if grep -qE "(^|[[:space:]])${param}($|[[:space:]])" "$f" 2>/dev/null; then
    log "Kernel param already present in: $f"
    return 0
  fi

  local backup="${f}.bak.wizado.$(date +%Y%m%d%H%M%S)"
  run_sudo cp -a "$f" "$backup"
  log "Backed up to: $backup"

  # systemd-boot entry format: "options <args...>"
  if grep -qE '^[[:space:]]*options[[:space:]]+' "$f"; then
    run_sudo sed -i -E "s/^[[:space:]]*options[[:space:]]+(.*)\$/options \\1 ${param}/" "$f"
  else
    # Append an options line if missing.
    run_sudo bash -lc "printf '\noptions %s\n' '${param}' >>'${f}'"
  fi
}

limine_enable_modeset() {
  local f="${LIMINE_CONF:-/boot/limine.conf}"
  [[ -f "$f" ]] || die "Limine config not found: $f"

  if grep -qE "(^|[[:space:]])nvidia-drm\\.modeset=1($|[[:space:]])" "$f" 2>/dev/null; then
    log "Kernel param already present in: $f"
    return 0
  fi

  local backup="${f}.bak.wizado.$(date +%Y%m%d%H%M%S)"
  run_sudo cp -a "$f" "$backup"
  log "Backed up to: $backup"

  # Append to every cmdline: line (covers default entry + snapshots). This is safe and keeps entries consistent.
  run_sudo sed -i -E 's/^([[:space:]]*cmdline:[[:space:]]*)(.*)$/\\1\\2 nvidia-drm.modeset=1/' "$f"
  log "Updated Limine cmdline entries in: $f"
}

enable_modeset_param() {
  if kernel_has_modeset; then
    log "Kernel already has nvidia-drm.modeset=1 (nothing to do)."
    return 0
  fi

  confirm_or_die "Add kernel parameter nvidia-drm.modeset=1 (required for NVIDIA TTY mode)?"

  limine_enable_modeset
}

main() {
  if ! parse_common_flags "$@"; then
    usage
    exit 0
  fi

  LIMINE_CONF="/boot/limine.conf"
  while [[ ${#REMAINING_ARGS[@]} -gt 0 ]]; do
    case "${REMAINING_ARGS[0]}" in
      --limine-conf)
        LIMINE_CONF="${REMAINING_ARGS[1]:-}"
        REMAINING_ARGS=("${REMAINING_ARGS[@]:2}")
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "Unknown arg: ${REMAINING_ARGS[0]}"
        ;;
    esac
  done

  require_cmd sudo
  require_cmd grep
  require_cmd sed
  require_cmd awk

  require_omarchy

  log "Enabling TTY prerequisites for wizado (NVIDIA)"
  ensure_sudoers_for_openvt
  enable_modeset_param

  warn "Reboot required for kernel parameter changes to take effect."
  log "After reboot: Super+Alt+S should enter true TTY gamescope mode."
}

main "$@"



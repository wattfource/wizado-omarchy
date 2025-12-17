#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"

usage() {
  cat <<'EOF'
Usage: ./scripts/remove.sh [--yes] [--dry-run]

Removes/undoes anything installed by setup.sh based on the state file.

Options:
  --yes, -y     Non-interactive; assume "yes" for confirmations.
  --dry-run     Print commands without running them.
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

remove_pacman_pkg() {
  local pkg="$1"
  # -Rns removes package + unneeded deps + config files owned by the package.
  run_sudo pacman -Rns ${ASSUME_YES:+--noconfirm} "$pkg"
}

remove_gamescope_cap() {
  local path="$1"
  [[ -n "$path" ]] || return 0
  if [[ ! -e "$path" ]]; then
    warn "setcap target not found (skipping): $path"
    return 0
  fi
  run_sudo setcap -r "$path"
}

remove_hypr_source_line() {
  local config_path="$1"
  [[ -n "$config_path" ]] || return 0
  if [[ ! -f "$config_path" ]]; then
    warn "Hypr config not found (skipping): $config_path"
    return 0
  fi
  # Remove only the line we added (marked).
  run sed -i '/the-wizard:source/d' "$config_path"
}

main() {
  if ! parse_common_flags "$@"; then
    usage
    exit 0
  fi

  require_cmd pacman

  log "Starting remove"
  confirm_or_die "This will remove items recorded by setup.sh. Continue?"

  local local_items
  local_items="$(read_installed_items || true)"
  if [[ -z "${local_items:-}" ]]; then
    warn "No state file entries found. Nothing to remove."
    exit 0
  fi

  local item pkg path config_path
  while IFS= read -r item; do
    [[ -n "$item" ]] || continue
    case "$item" in
      pacman:*)
        pkg="${item#pacman:}"
        log "Removing pacman package: $pkg"
        remove_pacman_pkg "$pkg"
        ;;
      file:*)
        path="${item#file:}"
        log "Removing file/dir: $path"
        if [[ -e "$path" ]] && [[ ! -w "$path" ]]; then
          run_sudo rm -rf -- "$path"
        else
          run rm -rf -- "$path"
        fi
        ;;
      setcap:*)
        path="${item#setcap:}"
        log "Removing setcap from: $path"
        remove_gamescope_cap "$path"
        ;;
      hyprsource:*)
        config_path="${item#hyprsource:}"
        log "Removing Hypr source line from: $config_path"
        remove_hypr_source_line "$config_path"
        ;;
      *)
        warn "Unknown state item type (skipping): $item"
        ;;
    esac
  done <<<"$local_items"

  log "Clearing state"
  if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
    clear_installed_items
  fi

  log "Remove completed"
}

main "$@"


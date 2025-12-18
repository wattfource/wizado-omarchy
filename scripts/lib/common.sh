#!/usr/bin/env bash
set -euo pipefail

PROJECT_NAME="wizado"

log() {
  # shellcheck disable=SC2059
  printf "[%s] %s\n" "$PROJECT_NAME" "$*"
}

warn() {
  # shellcheck disable=SC2059
  printf "[%s] WARNING: %s\n" "$PROJECT_NAME" "$*" >&2
}

die() {
  # shellcheck disable=SC2059
  printf "[%s] ERROR: %s\n" "$PROJECT_NAME" "$*" >&2
  exit 1
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || die "Missing required command: $cmd"
}

is_root() {
  [[ "${EUID:-$(id -u)}" -eq 0 ]]
}

require_root() {
  is_root || die "This action requires root. Re-run with sudo."
}

is_omarchy() {
  # Heuristic detection for Omarchy. We intentionally only support Omarchy setups.
  # - Omarchy installs its defaults under ~/.local/share/omarchy
  # - It also sets OMARCHY_PATH for some scripts
  [[ -d "${HOME}/.local/share/omarchy" ]] || [[ -n "${OMARCHY_PATH:-}" ]]
}

require_omarchy() {
  is_omarchy || die "This tool is designed for Omarchy (Arch Linux) only. Omarchy was not detected on this system."
}

repo_root() {
  # Resolve repo root based on this file location: scripts/lib/common.sh
  local here
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  (cd "$here/../.." && pwd)
}

state_dir() {
  # Local repo state (safe for scaffolding). For a packaged install, this will move.
  printf "%s/.state\n" "$(repo_root)"
}

state_file_installed_items() {
  printf "%s/installed_items.txt\n" "$(state_dir)"
}

ensure_state_dir() {
  mkdir -p "$(state_dir)"
}

record_installed_item() {
  # Record a line item to support remove.sh (packages, files, etc.)
  # Usage: record_installed_item "pacman:steam"
  local item="$1"
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: record_installed_item $item"
    return 0
  fi
  ensure_state_dir
  printf "%s\n" "$item" >>"$(state_file_installed_items)"
}

clear_installed_items() {
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: clear_installed_items"
    return 0
  fi
  ensure_state_dir
  : >"$(state_file_installed_items)"
}

read_installed_items() {
  local f
  f="$(state_file_installed_items)"
  [[ -f "$f" ]] || return 0
  cat "$f"
}

parse_common_flags() {
  # Sets globals:
  # - ASSUME_YES: 1 or 0
  # - DRY_RUN: 1 or 0
  ASSUME_YES=0
  DRY_RUN=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --yes|-y) ASSUME_YES=1; shift ;;
      --dry-run) DRY_RUN=1; shift ;;
      --help|-h) return 1 ;;
      *) break ;;
    esac
  done
  # shellcheck disable=SC2034
  REMAINING_ARGS=("$@")
}

confirm_or_die() {
  local prompt="$1"
  if [[ "${ASSUME_YES:-0}" -eq 1 ]]; then
    return 0
  fi
  read -r -p "$prompt [y/N]: " reply
  [[ "$reply" == "y" || "$reply" == "Y" ]] || die "Cancelled."
}

run() {
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: $*"
    return 0
  fi
  log "RUN: $*"
  "$@"
}

detect_aur_helper() {
  if command -v yay >/dev/null 2>&1; then
    echo "yay"
    return 0
  fi
  if command -v paru >/dev/null 2>&1; then
    echo "paru"
    return 0
  fi
  return 1
}


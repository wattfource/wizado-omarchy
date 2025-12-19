#!/usr/bin/env bash
# wizado: shared utilities

PROJECT_NAME="wizado"

log() {
  printf "[%s] %s\n" "$PROJECT_NAME" "$*"
}

warn() {
  printf "[%s] WARNING: %s\n" "$PROJECT_NAME" "$*" >&2
}

die() {
  printf "[%s] ERROR: %s\n" "$PROJECT_NAME" "$*" >&2
  exit 1
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || die "Missing required command: $cmd"
}

is_omarchy() {
  # Heuristic: Omarchy installs defaults under ~/.local/share/omarchy
  [[ -d "${HOME}/.local/share/omarchy" ]] || [[ -n "${OMARCHY_PATH:-}" ]]
}

require_omarchy() {
  is_omarchy || die "This tool is designed for Omarchy (Arch Linux) only"
}

parse_common_flags() {
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
}

confirm_or_die() {
  local prompt="$1"
  if [[ "${ASSUME_YES:-0}" -eq 1 ]]; then
    return 0
  fi
  read -r -p "$prompt [y/N]: " reply
  [[ "$reply" == "y" || "$reply" == "Y" ]] || die "Cancelled"
}

run() {
  if [[ "${DRY_RUN:-0}" -eq 1 ]]; then
    log "DRY-RUN: $*"
    return 0
  fi
  "$@"
}

#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"

usage() {
  cat <<'EOF'
Usage: ./scripts/update.sh [--yes] [--dry-run] [--system]

Updates packages / artifacts installed by setup.sh.

Options:
  --yes, -y     Non-interactive; assume "yes" for confirmations.
  --dry-run     Print commands without running them.
  --system      Run full system update (pacman -Syu). Without this flag, only
                updates packages recorded by setup.sh.
EOF
}

main() {
  local SYSTEM_UPDATE=0
  local -a forwarded=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --system) SYSTEM_UPDATE=1; shift ;;
      *) forwarded+=("$1"); shift ;;
    esac
  done

  if ! parse_common_flags "${forwarded[@]}"; then
    usage
    exit 0
  fi

  require_cmd pacman

  log "Starting update"
  confirm_or_die "This will update packages/artifacts. Continue?"

  if [[ "$SYSTEM_UPDATE" -eq 1 ]]; then
    require_root
    run pacman -Syu ${ASSUME_YES:+--noconfirm}
    log "Update completed (system)"
    return 0
  fi

  # Default: only update packages recorded by setup.sh.
  local -a pkgs=()
  local item
  while IFS= read -r item; do
    [[ -n "$item" ]] || continue
    case "$item" in
      pacman:*) pkgs+=("${item#pacman:}") ;;
    esac
  done < <(read_installed_items || true)

  if ((${#pkgs[@]} == 0)); then
    warn "No recorded pacman packages found in state. Nothing to update."
    log "Tip: run ./scripts/setup.sh first, or use ./scripts/update.sh --system"
    return 0
  fi

  require_root
  run pacman -S --needed ${ASSUME_YES:+--noconfirm} "${pkgs[@]}"

  log "Update completed"
}

main "$@"


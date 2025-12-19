#!/usr/bin/env bash
set -euo pipefail

# wizado remove: Uninstall gaming mode components

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

TARGET_DIR="${HOME}/.local/share/steam-launcher"
STATE_DIR="${HOME}/.cache/wizado"

usage() {
  cat <<'EOF'
wizado remove

Removes gaming mode launchers and Hyprland keybindings.

Options:
  --yes, -y     Non-interactive mode
  --dry-run     Print what would be done
  -h, --help    Show this help
EOF
}

remove_keybindings() {
  log "Removing Hyprland keybindings..."
  
  for config in \
      "$HOME/.config/hypr/bindings.conf" \
      "$HOME/.config/hypr/keybinds.conf" \
      "$HOME/.config/hypr/hyprland.conf"; do
    if [[ -f "$config" ]] && grep -q "# Gaming Mode bindings - added by wizado" "$config" 2>/dev/null; then
      log "Removing bindings from $config"
      if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
        sed -i '/# Gaming Mode bindings - added by wizado/,/# End Gaming Mode bindings/d' "$config"
      fi
    fi
  done
  
  # Also check for old wizado.conf include
  local include_file="$HOME/.config/hypr/conf.d/wizado.conf"
  if [[ -f "$include_file" ]]; then
    log "Removing $include_file"
    if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
      rm -f "$include_file"
    fi
  fi
  
  # Remove source line from hyprland.conf if present
  local main_config="$HOME/.config/hypr/hyprland.conf"
  if [[ -f "$main_config" ]] && grep -q "wizado" "$main_config" 2>/dev/null; then
    log "Removing wizado source line from $main_config"
    if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
      sed -i '/wizado/d' "$main_config"
    fi
  fi
}

remove_launchers() {
  log "Removing launcher scripts..."
  
  if [[ -d "$TARGET_DIR" ]]; then
    log "Removing $TARGET_DIR"
    if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
      rm -rf "$TARGET_DIR"
    fi
  fi
}

remove_state() {
  log "Removing state directory..."
  
  if [[ -d "$STATE_DIR" ]]; then
    log "Removing $STATE_DIR"
    if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
      rm -rf "$STATE_DIR"
    fi
  fi
}

maybe_remove_gamescope_cap() {
  command -v gamescope >/dev/null 2>&1 || return 0
  command -v getcap >/dev/null 2>&1 || return 0
  command -v setcap >/dev/null 2>&1 || return 0
  
  local gamescope_path
  gamescope_path="$(command -v gamescope)"
  
  if getcap "$gamescope_path" 2>/dev/null | grep -q 'cap_sys_nice'; then
    warn "Gamescope has cap_sys_nice capability"
    confirm_or_die "Remove cap_sys_nice from gamescope?"
    
    if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
      sudo setcap -r "$gamescope_path" || warn "Failed to remove capability"
    fi
    log "Removed cap_sys_nice from gamescope"
  fi
}

main() {
  if ! parse_common_flags "$@"; then
    usage
    exit 0
  fi

  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "  WIZADO - Remove Gaming Mode"
  echo "════════════════════════════════════════════════════════════════"
  echo ""

  confirm_or_die "Remove wizado gaming mode components?"

  remove_keybindings
  remove_launchers
  remove_state
  maybe_remove_gamescope_cap
  
  # Reload Hyprland
  if command -v hyprctl >/dev/null 2>&1; then
    hyprctl reload >/dev/null 2>&1 || true
  fi

  echo ""
  log "Removal complete"
  echo ""
  warn "Note: Steam and GPU drivers were NOT removed"
  warn "Remove them manually with: sudo pacman -Rns steam gamescope gamemode"
  echo ""
}

main "$@"

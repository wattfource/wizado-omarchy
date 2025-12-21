#!/usr/bin/env bash
set -euo pipefail

# wizado remove: Uninstall gaming mode components

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

LOCAL_BIN="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.config/wizado"
STATE_DIR="${HOME}/.cache/wizado"
SHARE_DIR="${HOME}/.local/share/wizado"
WAYBAR_DIR="${HOME}/.config/waybar"
WAYBAR_STYLE="${WAYBAR_DIR}/style.css"

usage() {
  cat <<'EOF'
wizado remove

Removes gaming mode launchers, keybindings, and waybar module.

Options:
  --yes, -y     Non-interactive mode
  --dry-run     Print what would be done
  --keep-config Keep configuration files
  -h, --help    Show this help
EOF
}

KEEP_CONFIG=0

remove_keybindings() {
  log "Removing Hyprland keybindings..."
  
  for config in \
      "$HOME/.config/hypr/bindings.conf" \
      "$HOME/.config/hypr/keybinds.conf" \
      "$HOME/.config/hypr/hyprland.conf"; do
    if [[ -f "$config" ]]; then
      # Remove all wizado binding blocks
      if grep -q "# Wizado - added by wizado" "$config" 2>/dev/null; then
        log "  Removing bindings from $config"
        if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
          sed -i '/# Wizado - added by wizado/,/# End Wizado bindings/d' "$config"
        fi
      fi
      # Also remove old-style bindings
      if grep -q "# Gaming Mode bindings - added by wizado" "$config" 2>/dev/null; then
        if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
          sed -i '/# Gaming Mode bindings - added by wizado/,/# End Gaming Mode bindings/d' "$config"
        fi
      fi
      if grep -q "# Steam - added by wizado" "$config" 2>/dev/null; then
        if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
          sed -i '/# Steam - added by wizado/,/# End Steam bindings/d' "$config"
        fi
      fi
    fi
  done
  
  # Also check for old wizado.conf include
  local include_file="$HOME/.config/hypr/conf.d/wizado.conf"
  if [[ -f "$include_file" ]]; then
    log "  Removing $include_file"
    if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
      rm -f "$include_file"
    fi
  fi
}

remove_waybar() {
  log "Removing Waybar configuration..."
  
  # Find waybar config (could be config or config.jsonc)
  local waybar_config=""
  if [[ -f "${WAYBAR_DIR}/config.jsonc" ]]; then
    waybar_config="${WAYBAR_DIR}/config.jsonc"
  elif [[ -f "${WAYBAR_DIR}/config" ]]; then
    waybar_config="${WAYBAR_DIR}/config"
  fi
  
  # Remove wizado from waybar config
  if [[ -n "$waybar_config" && -f "$waybar_config" ]]; then
    if grep -q '"custom/wizado"' "$waybar_config" 2>/dev/null; then
      log "  Removing wizado module from waybar config"
      if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
        # Remove module from modules-right array
        sed -i 's/"custom\/wizado",\s*//g' "$waybar_config" 2>/dev/null || true
        sed -i 's/,\s*"custom\/wizado"//g' "$waybar_config" 2>/dev/null || true
        
        # Remove module definition (try with jq if available)
        if command -v jq >/dev/null 2>&1; then
          local tmp_config
          tmp_config=$(mktemp)
          # Strip comments for jsonc and process
          if sed 's|//.*||g' "$waybar_config" | jq 'del(.["custom/wizado"])' > "$tmp_config" 2>/dev/null && [[ -s "$tmp_config" ]]; then
            mv "$tmp_config" "$waybar_config"
          else
            rm -f "$tmp_config"
            warn "  Could not remove module definition automatically"
          fi
        fi
      fi
    fi
  fi
  
  # Remove wizado styling from waybar style.css
  if [[ -f "$WAYBAR_STYLE" ]]; then
    if grep -q '#custom-wizado' "$WAYBAR_STYLE" 2>/dev/null; then
      log "  Removing wizado styling from waybar style.css"
      if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
        # Remove the wizado CSS block
        sed -i '/\/\* Wizado Waybar Module Styling/,/^$/d' "$WAYBAR_STYLE" 2>/dev/null || true
        sed -i '/#custom-wizado/,/^}/d' "$WAYBAR_STYLE" 2>/dev/null || true
      fi
    fi
  fi
  
  # Signal waybar to reload
  if pgrep -x waybar >/dev/null 2>&1; then
    pkill -SIGUSR2 waybar 2>/dev/null || true
    log "  Signaled waybar to reload"
  fi
}

remove_scripts() {
  log "Removing launcher scripts..."
  
  local scripts=(
    "wizado"
    "wizado-launch"
    "wizado-config"
    "wizado-waybar"
  )
  
  for script in "${scripts[@]}"; do
    if [[ -f "${LOCAL_BIN}/${script}" ]]; then
      log "  Removing ${LOCAL_BIN}/${script}"
      if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
        rm -f "${LOCAL_BIN}/${script}"
      fi
    fi
  done
  
  # Remove share directory (library files)
  if [[ -d "$SHARE_DIR" ]]; then
    log "  Removing $SHARE_DIR"
    if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
      rm -rf "$SHARE_DIR"
    fi
  fi
}

remove_config() {
  if [[ "$KEEP_CONFIG" -eq 1 ]]; then
    log "Keeping configuration (--keep-config)"
    return 0
  fi
  
  log "Removing configuration..."
  
  if [[ -d "$CONFIG_DIR" ]]; then
    log "  Removing $CONFIG_DIR"
    if [[ "${DRY_RUN:-0}" -eq 0 ]]; then
      rm -rf "$CONFIG_DIR"
    fi
  fi
}

remove_state() {
  log "Removing state/cache..."
  
  if [[ -d "$STATE_DIR" ]]; then
    log "  Removing $STATE_DIR"
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
  # Parse flags
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --keep-config) KEEP_CONFIG=1; shift ;;
      *) break ;;
    esac
  done
  
  if ! parse_common_flags "$@"; then
    usage
    exit 0
  fi

  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "  WIZADO - Remove Gaming Mode"
  echo "════════════════════════════════════════════════════════════════"
  echo ""
  
  echo "This will remove:"
  echo "  • Launcher scripts from ${LOCAL_BIN}"
  echo "  • Hyprland keybindings"
  echo "  • Waybar module"
  [[ "$KEEP_CONFIG" -eq 0 ]] && echo "  • Configuration at ${CONFIG_DIR}"
  echo "  • Cache at ${STATE_DIR}"
  echo ""

  confirm_or_die "Remove wizado gaming mode components?"

  remove_keybindings
  remove_waybar
  remove_scripts
  remove_config
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

#!/usr/bin/env bash
set -euo pipefail

# Waybar custom module for the-wizard.
# Intended config:
#  "custom/the-wizard": { "exec": "~/.config/waybar/scripts/the-wizard-status.sh", "return-type":"json", "interval":2, ... }

STATE_DIR="${HOME}/.cache/the-wizard"
MODE_FILE="${STATE_DIR}/mode"
VT_FILE="${STATE_DIR}/vt"

mode="nested"
vt=""
if [[ -f "$MODE_FILE" ]]; then
  mode="$(cat "$MODE_FILE" 2>/dev/null || echo "nested")"
fi
if [[ -f "$VT_FILE" ]]; then
  vt="$(cat "$VT_FILE" 2>/dev/null || true)"
fi

running="0"
if pgrep -x gamescope >/dev/null 2>&1 || pgrep -x steam >/dev/null 2>&1; then
  running="1"
fi

sudo_ready="0"
if sudo -n true >/dev/null 2>&1; then
  sudo_ready="1"
fi

icon=""
class="idle"
if [[ "$running" == "1" ]]; then
  class="running"
fi

mode_label="Normal (nested)"
if [[ "$mode" == "tty" ]]; then
  mode_label="Wizard (TTY)"
fi

tooltip=$(
  cat <<EOF
Steam Couch Mode

Status: ${class}
Mode: ${mode_label}${vt:+ (VT ${vt})}

Left click: Launch (normal)
Right click: Launch (wizard / no compositor)
Middle click: Exit (quit steam + gamescope)
EOF
)

if [[ "$mode" == "tty" && "$sudo_ready" != "1" ]]; then
  tooltip="${tooltip}"$'\n\n'"Note: wizard mode needs passwordless sudo for openvt/chvt."
fi

printf '{"text":"%s","alt":"%s","class":"%s","tooltip":"%s"}\n' \
  "$icon" \
  "$mode" \
  "$class" \
  "$(echo "$tooltip" | sed 's/"/\\"/g')"



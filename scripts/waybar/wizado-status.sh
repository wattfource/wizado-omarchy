#!/usr/bin/env bash
set -euo pipefail

# Waybar custom module for wizado.
# Intended config:
#  "custom/wizado": { "exec": "~/.config/waybar/scripts/wizado-status.sh", "return-type":"json", "interval":2, ... }

STATE_DIR="${HOME}/.cache/wizado"
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

icon="🎮"
class="idle"
if [[ "$running" == "1" ]]; then
  class="running"
fi

mode_label="Nested"
if [[ "$mode" == "performance" || "$mode" == "tty" ]]; then
  mode_label="Performance"
fi

tooltip_base="Steam Couch Mode

Status: ${class}
Mode: ${mode_label}${vt:+ (VT ${vt})}

Click to open menu"

if [[ "$mode" == "performance" || "$mode" == "tty" ]] && [[ "$sudo_ready" != "1" ]]; then
  tooltip_base="${tooltip_base}

Note: performance mode needs passwordless sudo for openvt/chvt."
fi

# Use Python for safe JSON encoding if available, otherwise fall back to minimal sed (risky for newlines)
if command -v python3 >/dev/null 2>&1; then
  python3 -c "import json, sys; print(json.dumps({'text': sys.argv[1], 'alt': sys.argv[2], 'class': sys.argv[3], 'tooltip': sys.argv[4]}))" \
    "$icon" "$mode" "$class" "$tooltip_base"
else
  # Minimal fallback: escape backslashes, quotes, and newlines
  # 1. Escape backslashes
  # 2. Escape double quotes
  # 3. Replace actual newlines with \n
  json_tooltip="$(echo "$tooltip_base" | sed 's/\\/\\\\/g; s/"/\\"/g' | awk '{printf "%s\\n", $0}' | sed 's/\\n$//')"
  printf '{"text":"%s","alt":"%s","class":"%s","tooltip":"%s"}\n' \
    "$icon" \
    "$mode" \
    "$class" \
    "$json_tooltip"
fi

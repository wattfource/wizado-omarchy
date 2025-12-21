#!/usr/bin/env bash
# wizado: License TUI using Gum

WIZADO_URL="https://wizado.app"

# Check if gum is available
_has_gum() {
  command -v gum >/dev/null 2>&1
}

# Fallback to basic prompts if gum not available
_fallback_input() {
  local prompt="$1"
  local value
  read -r -p "$prompt: " value
  echo "$value"
}

_fallback_confirm() {
  local prompt="$1"
  local reply
  read -r -p "$prompt [y/N]: " reply
  [[ "$reply" == "y" || "$reply" == "Y" ]]
}

# Display the wizado banner
show_banner() {
  if _has_gum; then
    gum style \
      --border double \
      --border-foreground 212 \
      --padding "1 3" \
      --margin "1" \
      --align center \
      "🎮 WIZADO" \
      "Steam Gaming Mode"
  else
    echo ""
    echo "╔═══════════════════════════════════════════╗"
    echo "║          🎮 WIZADO                        ║"
    echo "║       Steam Gaming Mode                   ║"
    echo "╚═══════════════════════════════════════════╝"
    echo ""
  fi
}

# Show license required message
show_license_required() {
  if _has_gum; then
    gum style \
      --foreground 214 \
      --bold \
      "License Required"
    echo ""
    gum style \
      --foreground 250 \
      "Wizado requires a valid license to run." \
      "" \
      "If you have a license key, enter it below." \
      "If you need to purchase a license, select 'Get License'."
    echo ""
    gum style \
      --foreground 39 \
      --italic \
      "\$5 for 5 machines at wizado.app"
  else
    echo ""
    echo "═══════════════════════════════════════════"
    echo "  LICENSE REQUIRED"
    echo "═══════════════════════════════════════════"
    echo ""
    echo "  Wizado requires a valid license to run."
    echo ""
    echo "  \$5 for 5 machines at wizado.app"
    echo ""
    echo "  If you have a license key, enter it below."
    echo "  If you need to purchase a license, visit:"
    echo "    $WIZADO_URL"
    echo ""
  fi
}

# Show license expired/invalid message
show_license_invalid() {
  local reason="${1:-invalid}"
  local message
  
  case "$reason" in
    "invalid")
      message="Your license is invalid or has been revoked."
      ;;
    "expired")
      message="Your license has expired."
      ;;
    "machine_mismatch")
      message="This license is activated on a different machine."
      ;;
    "offline_expired")
      message="Cannot verify license (offline) and grace period expired."
      ;;
    *)
      message="License validation failed."
      ;;
  esac
  
  if _has_gum; then
    gum style \
      --foreground 196 \
      --bold \
      "License Error"
    echo ""
    gum style \
      --foreground 250 \
      "$message" \
      "" \
      "Please re-enter your license key or contact support."
  else
    echo ""
    echo "═══════════════════════════════════════════"
    echo "  LICENSE ERROR"
    echo "═══════════════════════════════════════════"
    echo ""
    echo "  $message"
    echo ""
    echo "  Please re-enter your license key or contact support."
    echo ""
  fi
}

# Prompt user to enter license key
# Returns the entered license key, or empty if cancelled
prompt_license_key() {
  echo ""
  
  if _has_gum; then
    # Show options
    local choice
    choice=$(gum choose \
      --header "What would you like to do?" \
      --cursor.foreground 212 \
      "Enter License Key" \
      "Get License (opens $WIZADO_URL)" \
      "Exit")
    
    case "$choice" in
      "Enter License Key")
        local license
        license=$(gum input \
          --placeholder "XXXX-XXXX-XXXX-XXXX" \
          --header "Enter your license key:" \
          --width 40 \
          --char-limit 50)
        echo "$license"
        ;;
      "Get License"*)
        show_purchase_url
        return 1
        ;;
      "Exit"|"")
        return 2
        ;;
    esac
  else
    echo "Options:"
    echo "  1) Enter License Key"
    echo "  2) Get License (visit $WIZADO_URL)"
    echo "  3) Exit"
    echo ""
    read -r -p "Select option [1-3]: " opt
    
    case "$opt" in
      1)
        read -r -p "Enter license key: " license
        echo "$license"
        ;;
      2)
        show_purchase_url
        return 1
        ;;
      *)
        return 2
        ;;
    esac
  fi
}

# Show the purchase URL
show_purchase_url() {
  echo ""
  
  if _has_gum; then
    gum style \
      --border rounded \
      --border-foreground 39 \
      --padding "1 2" \
      --margin "1" \
      "Get a Wizado License" \
      "" \
      "\$5 for 5 machines" \
      "" \
      "$WIZADO_URL"
    
    echo ""
    gum style \
      --foreground 250 \
      --italic \
      "Your license key will be emailed after purchase."
    
    # Offer to open in browser
    if command -v xdg-open >/dev/null 2>&1; then
      echo ""
      if gum confirm "Open in browser?"; then
        xdg-open "$WIZADO_URL" 2>/dev/null &
      fi
    fi
  else
    echo "═══════════════════════════════════════════"
    echo "  GET A LICENSE"
    echo "═══════════════════════════════════════════"
    echo ""
    echo "  \$5 for 5 machines"
    echo ""
    echo "  Visit: $WIZADO_URL"
    echo ""
    echo "  Your license key will be emailed after purchase."
    echo ""
    
    if command -v xdg-open >/dev/null 2>&1; then
      read -r -p "Open in browser? [y/N]: " reply
      if [[ "$reply" == "y" || "$reply" == "Y" ]]; then
        xdg-open "$WIZADO_URL" 2>/dev/null &
      fi
    fi
  fi
}

# Show activation success
show_activation_success() {
  local email="${1:-}"
  local slots_used="${2:-}"
  local slots_total="${3:-}"
  
  echo ""
  
  if _has_gum; then
    gum style \
      --foreground 82 \
      --bold \
      "✓ License Activated!"
    
    if [[ -n "$email" ]]; then
      echo ""
      gum style \
        --foreground 250 \
        "Registered to: $email"
    fi
    
    if [[ -n "$slots_used" && -n "$slots_total" ]]; then
      gum style \
        --foreground 250 \
        "Activations: $slots_used / $slots_total"
    fi
    
    echo ""
    gum style \
      --foreground 250 \
      "Starting Steam Gaming Mode..."
    
    # Brief pause to show message
    sleep 2
  else
    echo "═══════════════════════════════════════════"
    echo "  ✓ LICENSE ACTIVATED!"
    echo "═══════════════════════════════════════════"
    echo ""
    [[ -n "$email" ]] && echo "  Registered to: $email"
    [[ -n "$slots_used" && -n "$slots_total" ]] && echo "  Activations: $slots_used / $slots_total"
    echo ""
    echo "  Starting Steam Gaming Mode..."
    echo ""
    sleep 2
  fi
}

# Show activation failure
show_activation_failure() {
  local message="${1:-Activation failed}"
  
  echo ""
  
  if _has_gum; then
    gum style \
      --foreground 196 \
      --bold \
      "✗ Activation Failed"
    echo ""
    gum style \
      --foreground 250 \
      "$message"
  else
    echo "═══════════════════════════════════════════"
    echo "  ✗ ACTIVATION FAILED"
    echo "═══════════════════════════════════════════"
    echo ""
    echo "  $message"
    echo ""
  fi
}

# Show offline grace period warning
show_offline_warning() {
  local days_remaining="${1:-unknown}"
  
  if _has_gum; then
    gum style \
      --foreground 214 \
      --italic \
      "⚠ Offline mode: License will need verification in $days_remaining days"
  else
    echo "⚠ Offline mode: License will need verification in $days_remaining days"
  fi
  echo ""
}

# Show a spinner while doing something
show_spinner() {
  local title="$1"
  shift
  
  if _has_gum; then
    gum spin --spinner dot --title "$title" -- "$@"
  else
    echo "$title..."
    "$@"
  fi
}

# Show verification in progress
show_verifying() {
  if _has_gum; then
    gum style \
      --foreground 250 \
      --italic \
      "Verifying license..."
  else
    echo "Verifying license..."
  fi
}

# Main license prompt flow
# Returns: 0 if license obtained, 1 if user wants to purchase, 2 if user cancelled
run_license_prompt() {
  local reason="${1:-}"
  
  clear
  show_banner
  
  if [[ -n "$reason" && "$reason" != "no_license" ]]; then
    show_license_invalid "$reason"
  else
    show_license_required
  fi
  
  echo ""
  prompt_license_key
}


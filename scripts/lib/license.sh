#!/usr/bin/env bash

LICENSE_CONFIG_DIR="${HOME}/.config/wizado"
LICENSE_FILE="${LICENSE_CONFIG_DIR}/license"
PAYMENT_URL="https://example.com/buy-wizado" # TODO: Update with actual URL
VERIFY_URL="https://example.com/api/verify"  # TODO: Update with actual URL

check_license() {
  local dry_run="${DRY_RUN:-0}"
  
  if [[ "$dry_run" -eq 1 ]]; then
    echo "DRY-RUN: Skipping license check."
    return 0
  fi

  # Check for existing license
  if [[ -f "$LICENSE_FILE" ]]; then
    local stored_key
    stored_key="$(cat "$LICENSE_FILE" | tr -d '[:space:]')"
    if verify_key_remote "$stored_key"; then
      return 0
    else
      echo "Stored license key is invalid."
    fi
  fi

  # Prompt for license
  echo "========================================================"
  echo "  Wizado License Check"
  echo "========================================================"
  echo "  This software requires a license ($5)."
  echo "  Please purchase a key at: $PAYMENT_URL"
  echo "========================================================"

  while true; do
    read -rp "Enter your license key: " user_key
    user_key="$(echo "$user_key" | tr -d '[:space:]')"

    if [[ -z "$user_key" ]]; then
      echo "Key cannot be empty."
      continue
    fi

    if verify_key_remote "$user_key"; then
      echo "License verified!"
      mkdir -p "$LICENSE_CONFIG_DIR"
      echo "$user_key" > "$LICENSE_FILE"
      return 0
    fi
    
    echo "Invalid license key. Please check and try again."
    echo "Buy a key at: $PAYMENT_URL"
  done
}

verify_key_remote() {
  local key="$1"
  
  # TODO: Replace with actual server-side verification logic.
  # Example: curl -sSf "${VERIFY_URL}?key=${key}" >/dev/null
  
  # For development/demonstration, allow a specific key
  if [[ "$key" == "DEV-KEY-12345" ]]; then
    return 0
  fi

  # Fail by default if not implemented
  return 1
}


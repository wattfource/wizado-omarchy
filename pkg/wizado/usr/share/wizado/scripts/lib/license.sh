#!/usr/bin/env bash
# wizado: License validation functions

# Configuration
WIZADO_API_URL="${WIZADO_API_URL:-https://wizado.app/api}"
LICENSE_DIR="${HOME}/.config/wizado"
LICENSE_FILE="${LICENSE_DIR}/license.json"
GRACE_PERIOD_DAYS=14
REVERIFY_DAYS=7
API_TIMEOUT=5

# Ensure jq is available (fallback to basic parsing if not)
_has_jq() {
  command -v jq >/dev/null 2>&1
}

# Generate a unique machine ID based on hardware
generate_machine_id() {
  local machine_id=""
  
  # Try multiple sources for hardware identification
  if [[ -f /etc/machine-id ]]; then
    machine_id+=$(cat /etc/machine-id)
  fi
  
  # Add CPU info
  if [[ -f /proc/cpuinfo ]]; then
    machine_id+=$(grep -m1 "model name" /proc/cpuinfo 2>/dev/null | cut -d: -f2 || true)
  fi
  
  # Add hostname
  machine_id+=$(hostname 2>/dev/null || echo "unknown")
  
  # Add username for per-user uniqueness
  machine_id+="$USER"
  
  # Hash it all
  echo -n "$machine_id" | sha256sum | cut -d' ' -f1
}

# Read a value from the license JSON file
_read_license_field() {
  local field="$1"
  
  [[ -f "$LICENSE_FILE" ]] || return 1
  
  if _has_jq; then
    jq -r ".$field // empty" "$LICENSE_FILE" 2>/dev/null
  else
    # Basic grep fallback
    grep -oP "\"$field\"\\s*:\\s*\"\\K[^\"]*" "$LICENSE_FILE" 2>/dev/null | head -1
  fi
}

# Write the license file
_write_license_file() {
  local license="$1"
  local machine_id="$2"
  local email="${3:-}"
  local now
  now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  
  mkdir -p "$LICENSE_DIR"
  chmod 700 "$LICENSE_DIR"
  
  cat > "$LICENSE_FILE" <<EOF
{
  "license": "$license",
  "machineId": "$machine_id",
  "activatedAt": "$now",
  "lastVerified": "$now",
  "email": "$email"
}
EOF
  chmod 600 "$LICENSE_FILE"
}

# Update the lastVerified timestamp
_update_verified_timestamp() {
  local now
  now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  
  if _has_jq && [[ -f "$LICENSE_FILE" ]]; then
    local tmp
    tmp=$(mktemp)
    jq ".lastVerified = \"$now\"" "$LICENSE_FILE" > "$tmp" && mv "$tmp" "$LICENSE_FILE"
    chmod 600 "$LICENSE_FILE"
  else
    # Fallback: sed replace
    sed -i "s/\"lastVerified\":.*/\"lastVerified\": \"$now\",/" "$LICENSE_FILE" 2>/dev/null || true
  fi
}

# Calculate days since a timestamp
_days_since() {
  local timestamp="$1"
  local then_epoch now_epoch
  
  # Parse ISO timestamp to epoch
  then_epoch=$(date -d "$timestamp" +%s 2>/dev/null || echo 0)
  now_epoch=$(date +%s)
  
  echo $(( (now_epoch - then_epoch) / 86400 ))
}

# Check if we're within the grace period
_within_grace_period() {
  local last_verified
  last_verified=$(_read_license_field "lastVerified")
  
  [[ -n "$last_verified" ]] || return 1
  
  local days_since
  days_since=$(_days_since "$last_verified")
  
  [[ $days_since -lt $GRACE_PERIOD_DAYS ]]
}

# Check if re-verification is needed
_needs_reverification() {
  local last_verified
  last_verified=$(_read_license_field "lastVerified")
  
  [[ -n "$last_verified" ]] || return 0
  
  local days_since
  days_since=$(_days_since "$last_verified")
  
  [[ $days_since -ge $REVERIFY_DAYS ]]
}

# Call the license verification API
# Returns: 0 if valid, 1 if invalid, 2 if network error
verify_license_api() {
  local email="$1"
  local license="$2"
  local response http_code body
  
  # Make the API call
  response=$(curl -s -w "\n%{http_code}" \
    --connect-timeout "$API_TIMEOUT" \
    --max-time "$((API_TIMEOUT * 2))" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "{\"email\": \"$email\", \"license\": \"$license\"}" \
    "${WIZADO_API_URL}/license/verify" 2>/dev/null) || {
    return 2  # Network error
  }
  
  # Split response and HTTP code
  http_code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | sed '$d')
  
  # Check HTTP status
  if [[ "$http_code" -ge 500 ]]; then
    return 2  # Server error, treat as network issue
  fi
  
  # Parse response
  local valid
  if _has_jq; then
    valid=$(echo "$body" | jq -r '.valid // false')
  else
    valid=$(echo "$body" | grep -oP '"valid"\s*:\s*\K(true|false)' | head -1)
  fi
  
  if [[ "$valid" == "true" ]]; then
    return 0
  else
    return 1
  fi
}

# Call the license activation API
# Returns: 0 if activated, 1 if failed
# Sets: ACTIVATION_MESSAGE, ACTIVATION_EMAIL, SLOTS_USED, SLOTS_TOTAL
activate_license_api() {
  local email="$1"
  local license="$2"
  local machine_id="$3"
  local response http_code body
  
  ACTIVATION_MESSAGE=""
  ACTIVATION_EMAIL=""
  SLOTS_USED=""
  SLOTS_TOTAL=""
  
  # Make the API call
  response=$(curl -s -w "\n%{http_code}" \
    --connect-timeout "$API_TIMEOUT" \
    --max-time "$((API_TIMEOUT * 2))" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "{\"email\": \"$email\", \"license\": \"$license\", \"machineId\": \"$machine_id\"}" \
    "${WIZADO_API_URL}/license/activate" 2>/dev/null) || {
    ACTIVATION_MESSAGE="Network error: Could not reach license server"
    return 1
  }
  
  # Split response and HTTP code
  http_code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | sed '$d')
  
  # Parse response
  if _has_jq; then
    local activated
    activated=$(echo "$body" | jq -r '.activated // false')
    ACTIVATION_MESSAGE=$(echo "$body" | jq -r '.message // .error // "Unknown error"')
    ACTIVATION_EMAIL=$(echo "$body" | jq -r '.email // empty')
    SLOTS_USED=$(echo "$body" | jq -r '.slotsUsed // empty')
    SLOTS_TOTAL=$(echo "$body" | jq -r '.slotsTotal // empty')
    
    if [[ "$activated" == "true" ]]; then
      return 0
    fi
  else
    # Basic parsing fallback
    if echo "$body" | grep -q '"activated"\s*:\s*true'; then
      ACTIVATION_MESSAGE="License activated successfully"
      return 0
    fi
    ACTIVATION_MESSAGE=$(echo "$body" | grep -oP '"message"\s*:\s*"\K[^"]*' | head -1)
    ACTIVATION_MESSAGE="${ACTIVATION_MESSAGE:-$(echo "$body" | grep -oP '"error"\s*:\s*"\K[^"]*' | head -1)}"
    ACTIVATION_MESSAGE="${ACTIVATION_MESSAGE:-License activation failed}"
  fi
  
  return 1
}

# Clear the stored license
clear_license() {
  rm -f "$LICENSE_FILE"
}

# Get stored license key
get_stored_license() {
  _read_license_field "license"
}

# Get stored email
get_stored_email() {
  _read_license_field "email"
}

# Get stored machine ID
get_stored_machine_id() {
  _read_license_field "machineId"
}

# Main license check function
# Returns: 0 if licensed and valid, 1 if not licensed, 2 if needs activation
# Sets: LICENSE_STATUS with detailed status
check_license() {
  LICENSE_STATUS=""
  
  local stored_email stored_license stored_machine_id current_machine_id
  stored_email=$(get_stored_email)
  stored_license=$(get_stored_license)
  stored_machine_id=$(get_stored_machine_id)
  current_machine_id=$(generate_machine_id)
  
  # No license stored
  if [[ -z "$stored_license" || -z "$stored_email" ]]; then
    LICENSE_STATUS="no_license"
    return 1
  fi
  
  # Machine ID mismatch (moved to different machine)
  if [[ "$stored_machine_id" != "$current_machine_id" ]]; then
    LICENSE_STATUS="machine_mismatch"
    return 2
  fi
  
  # Check if we need to re-verify
  if _needs_reverification; then
    # Try to verify online
    verify_license_api "$stored_email" "$stored_license"
    local verify_result=$?
    
    case $verify_result in
      0)
        # Valid - update timestamp
        _update_verified_timestamp
        LICENSE_STATUS="valid"
        return 0
        ;;
      1)
        # Invalid - license revoked or expired
        LICENSE_STATUS="invalid"
        clear_license
        return 1
        ;;
      2)
        # Network error - check grace period
        if _within_grace_period; then
          LICENSE_STATUS="offline_grace"
          return 0
        else
          LICENSE_STATUS="offline_expired"
          return 1
        fi
        ;;
    esac
  fi
  
  # Within re-verify window, check grace period
  if _within_grace_period; then
    LICENSE_STATUS="valid"
    return 0
  else
    LICENSE_STATUS="expired"
    return 1
  fi
}

# Activate a new license
# Returns: 0 on success, 1 on failure
# Sets: LICENSE_STATUS, ACTIVATION_MESSAGE
activate_license() {
  local email="$1"
  local license="$2"
  local machine_id
  machine_id=$(generate_machine_id)
  
  if activate_license_api "$email" "$license" "$machine_id"; then
    _write_license_file "$license" "$machine_id" "$email"
    LICENSE_STATUS="activated"
    return 0
  else
    LICENSE_STATUS="activation_failed"
    return 1
  fi
}


#!/usr/bin/env bash
set -euo pipefail

# Wizado - One-liner installer
# Usage: curl -fsSL https://raw.githubusercontent.com/REPLACE_ME/wizado/main/install.sh | bash
#
# This script:
# 1. Clones the wizado repo to ~/.local/share/wizado-src/
# 2. Runs the setup script
# 3. Optionally removes the source after install

REPO_URL="${WIZADO_REPO:-https://github.com/REPLACE_ME/wizado.git}"
INSTALL_DIR="${HOME}/.local/share/wizado-src"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() { echo -e "${BLUE}[wizado]${NC} $*"; }
success() { echo -e "${GREEN}[wizado]${NC} $*"; }
warn() { echo -e "${YELLOW}[wizado]${NC} $*"; }
error() { echo -e "${RED}[wizado]${NC} $*" >&2; }

die() {
  error "$1"
  exit 1
}

# Check prerequisites
check_prerequisites() {
  log "Checking prerequisites..."
  
  command -v git >/dev/null 2>&1 || die "git is required but not installed"
  command -v pacman >/dev/null 2>&1 || die "This installer is for Arch Linux (pacman not found)"
  
  # Check if Hyprland is available
  if ! command -v hyprctl >/dev/null 2>&1; then
    warn "Hyprland not detected. Wizado requires Hyprland."
    warn "Install Hyprland first, or continue at your own risk."
    read -r -p "Continue anyway? [y/N]: " reply
    [[ "$reply" == "y" || "$reply" == "Y" ]] || exit 0
  fi
  
  success "Prerequisites OK"
}

# Clone or update the repository
clone_repo() {
  log "Setting up wizado source..."
  
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    log "Existing installation found, updating..."
    cd "$INSTALL_DIR"
    git fetch origin
    git reset --hard origin/main 2>/dev/null || git reset --hard origin/master
    success "Updated to latest version"
  else
    log "Cloning wizado repository..."
    rm -rf "$INSTALL_DIR"
    mkdir -p "$(dirname "$INSTALL_DIR")"
    git clone --depth 1 "$REPO_URL" "$INSTALL_DIR" || die "Failed to clone repository"
    success "Repository cloned"
  fi
}

# Run the setup script
run_setup() {
  log "Running wizado setup..."
  cd "$INSTALL_DIR"
  
  if [[ ! -f "./scripts/setup.sh" ]]; then
    die "Setup script not found. Repository may be corrupted."
  fi
  
  chmod +x ./scripts/setup.sh
  ./scripts/setup.sh "$@"
}

# Main
main() {
  echo ""
  echo "╔═══════════════════════════════════════════════════════════════╗"
  echo "║                    WIZADO INSTALLER                           ║"
  echo "║              Steam Gaming Mode for Hyprland                   ║"
  echo "╚═══════════════════════════════════════════════════════════════╝"
  echo ""
  
  check_prerequisites
  clone_repo
  run_setup "$@"
  
  echo ""
  success "Wizado installation complete!"
  echo ""
  echo "  Source location: $INSTALL_DIR"
  echo ""
  echo "  To update in the future:"
  echo "    cd $INSTALL_DIR && git pull && ./scripts/setup.sh"
  echo ""
  echo "  To uninstall:"
  echo "    wizado remove"
  echo "    rm -rf $INSTALL_DIR"
  echo ""
}

main "$@"


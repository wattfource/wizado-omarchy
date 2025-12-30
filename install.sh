#!/usr/bin/env bash
set -euo pipefail

# Wizado - One-liner installer
# Usage: curl -fsSL https://wizado.app/install.sh | bash
#
# This script builds and installs wizado from source.
# Requires: go >= 1.21, git

REPO_URL="${WIZADO_REPO:-https://github.com/wattfource/wizado-omarchy.git}"
INSTALL_DIR="${HOME}/.cache/wizado-build"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[wizado]${NC} $*"; }
success() { echo -e "${GREEN}[wizado]${NC} $*"; }
warn() { echo -e "${YELLOW}[wizado]${NC} $*"; }
error() { echo -e "${RED}[wizado]${NC} $*" >&2; }

die() {
  error "$1"
  exit 1
}

check_prerequisites() {
  log "Checking prerequisites..."
  
  command -v git >/dev/null 2>&1 || die "git is required but not installed"
  command -v go >/dev/null 2>&1 || die "go is required but not installed (sudo pacman -S go)"
  
  # Check Go version
  GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
  if [[ "$(printf '%s\n' "1.21" "$GO_VERSION" | sort -V | head -n1)" != "1.21" ]]; then
    die "Go 1.21+ required, found $GO_VERSION"
  fi
  
  if ! command -v hyprctl >/dev/null 2>&1; then
    warn "Hyprland not detected. Wizado requires Hyprland."
  fi
  
  success "Prerequisites OK (Go $GO_VERSION)"
}

clone_repo() {
  log "Fetching wizado source..."
  
  rm -rf "$INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  git clone --depth 1 "$REPO_URL" "$INSTALL_DIR" || die "Failed to clone repository"
  
  success "Source fetched"
}

build_and_install() {
  log "Building wizado..."
  cd "$INSTALL_DIR"
  
  make build || die "Build failed"
  
  log "Installing wizado..."
  sudo install -Dm755 wizado /usr/bin/wizado || die "Install failed (need sudo)"
  
  # Install helper script for floating terminal launch
  sudo install -Dm755 scripts/bin/wizado-menu-float /usr/bin/wizado-menu-float
  
  # Install config files
  sudo install -Dm644 scripts/config/default.conf /usr/share/wizado/default.conf
  sudo install -Dm644 scripts/config/waybar-module.jsonc /usr/share/wizado/waybar-module.jsonc
  sudo install -Dm644 scripts/config/waybar-style.css /usr/share/wizado/waybar-style.css
  
  success "Wizado installed to /usr/bin/wizado"
}

cleanup() {
  log "Cleaning up..."
  rm -rf "$INSTALL_DIR"
}

main() {
  echo ""
  echo "╔═══════════════════════════════════════════════════════════════╗"
  echo "║                    WIZADO INSTALLER                           ║"
  echo "║              Steam Gaming Mode for Hyprland                   ║"
  echo "╚═══════════════════════════════════════════════════════════════╝"
  echo ""
  
  check_prerequisites
  clone_repo
  build_and_install
  cleanup
  
  echo ""
  success "Wizado installation complete!"
  echo ""
  echo "  Next steps:"
  echo "    wizado setup      # Configure system (install deps, keybinds)"
  echo "    wizado config     # Enter license and configure settings"
  echo ""
  echo "  License required: \$10 for 5 machines at https://wizado.app"
  echo ""
  echo "  To uninstall:"
  echo "    wizado remove"
  echo "    sudo rm /usr/bin/wizado"
  echo ""
}

main "$@"

#!/usr/bin/env bash
set -euo pipefail

# Setup auto-login gaming session on TTY3
# Run with: sudo ./setup-gaming-tty.sh

USER_NAME="${1:-seanfournier}"
GAMING_TTY="tty3"

echo "Setting up auto-login gaming session on $GAMING_TTY for user $USER_NAME"

# Create systemd override directory for getty@tty3
mkdir -p /etc/systemd/system/getty@${GAMING_TTY}.service.d

# Create autologin override
cat > /etc/systemd/system/getty@${GAMING_TTY}.service.d/autologin.conf << EOF
[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin $USER_NAME --noclear %I \$TERM
EOF

# Enable getty@tty3
systemctl enable getty@${GAMING_TTY}.service

# Reload systemd
systemctl daemon-reload

echo ""
echo "Done! TTY3 will auto-login as $USER_NAME"
echo ""
echo "Now add this to your ~/.bash_profile (or ~/.zprofile for zsh):"
echo ""
echo '  # Auto-launch Steam on TTY3'
echo '  if [[ "$(tty)" == "/dev/tty3" ]]; then'
echo '    exec ~/.local/bin/wizado-tty'
echo '  fi'
echo ""


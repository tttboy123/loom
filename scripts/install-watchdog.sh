#!/usr/bin/env bash
# =============================================================================
# scripts/install-watchdog.sh
#
# Install the Loom watchdog as a managed background service:
#   - macOS  → LaunchAgent (~/Library/LaunchAgents/com.loom.watchdog.plist)
#   - Linux  → systemd user timer + service
#
# Usage:
#   scripts/install-watchdog.sh install   # install + start
#   scripts/install-watchdog.sh uninstall # stop + remove
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WATCHDOG="$ROOT/scripts/loom-watchdog.sh"

action="${1:-install}"

install_macos() {
  local plist_dir="$HOME/Library/LaunchAgents"
  local plist="$plist_dir/com.loom.watchdog.plist"
  mkdir -p "$plist_dir"

  cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.loom.watchdog</string>

  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>$WATCHDOG</string>
    <string>tick</string>
  </array>

  <key>EnvironmentVariables</key>
  <dict>
    <key>LOOM_ROOT</key>
    <string>$ROOT</string>
  </dict>

  <key>RunAtLoad</key>
  <true/>

  <key>StartInterval</key>
  <integer>60</integer>

  <key>StandardOutPath</key>
  <string>$ROOT/devkit/logs/watchdog.plist.out</string>

  <key>StandardErrorPath</key>
  <string>$ROOT/devkit/logs/watchdog.plist.err</string>

  <key>ProcessType</key>
  <string>Background</string>
</dict>
</plist>
EOF

  echo "→ Wrote $plist"
  launchctl unload "$plist" 2>/dev/null || true
  launchctl load -w "$plist"
  echo "→ Loaded via launchctl"
  echo
  echo "Verify with:  launchctl list | grep loom"
  echo "Tail logs:    tail -F $ROOT/devkit/logs/watchdog.log"
}

install_linux() {
  local systemd_user_dir="$HOME/.config/systemd/user"
  mkdir -p "$systemd_user_dir"

  cat > "$systemd_user_dir/loom-watchdog.service" <<EOF
[Unit]
Description=Loom autopilot watchdog
After=network.target

[Service]
Type=oneshot
ExecStart=/bin/bash $WATCHDOG tick
WorkingDirectory=$ROOT
EOF

  cat > "$systemd_user_dir/loom-watchdog.timer" <<EOF
[Unit]
Description=Run Loom watchdog every 60 seconds

[Timer]
OnBootSec=30
OnUnitActiveSec=60
Persistent=true

[Install]
WantedBy=timers.target
EOF

  systemctl --user daemon-reload
  systemctl --user enable --now loom-watchdog.timer
  echo "→ Installed loom-watchdog.service + .timer (user systemd)"
  echo
  echo "Verify with:  systemctl --user status loom-watchdog.timer"
  echo "Tail logs:    journalctl --user -u loom-watchdog.service -f"
}

uninstall_macos() {
  local plist="$HOME/Library/LaunchAgents/com.loom.watchdog.plist"
  if [[ -f "$plist" ]]; then
    launchctl unload "$plist" 2>/dev/null || true
    /Users/lune/.mavis/bin/mavis-trash -- "$plist"
    echo "→ Removed $plist"
  else
    echo "→ Not installed (no $plist)"
  fi
}

uninstall_linux() {
  systemctl --user disable --now loom-watchdog.timer 2>/dev/null || true
  /Users/lune/.mavis/bin/mavis-trash -- "$HOME/.config/systemd/user/loom-watchdog.service"
  /Users/lune/.mavis/bin/mavis-trash -- "$HOME/.config/systemd/user/loom-watchdog.timer"
  systemctl --user daemon-reload
  echo "→ Removed systemd units"
}

case "$action" in
  install)
    echo "Installing Loom watchdog..."
    if [[ "$(uname)" == "Darwin" ]]; then
      install_macos
    else
      install_linux
    fi
    ;;
  uninstall)
    echo "Uninstalling Loom watchdog..."
    if [[ "$(uname)" == "Darwin" ]]; then
      uninstall_macos
    else
      uninstall_linux
    fi
    ;;
  *)
    echo "Usage: $0 [install|uninstall]"
    exit 2
    ;;
esac
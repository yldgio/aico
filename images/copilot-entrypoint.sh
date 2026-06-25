#!/bin/sh
# copilot-entrypoint.sh — starts dbus + gnome-keyring headlessly, then exec's copilot.
# Installed at /usr/local/bin/copilot-entrypoint.sh in the aico image.
#
# This script is idempotent: if dbus or keyring are already running (e.g. on
# container resume / `docker start`), it reuses the existing session rather than
# spawning duplicates.
set -e

# ── 1. D-Bus session bus ─────────────────────────────────────────────────────
if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then
  if command -v dbus-launch >/dev/null 2>&1; then
    eval "$(dbus-launch --sh-syntax 2>/dev/null)" || true
    export DBUS_SESSION_BUS_ADDRESS
  fi
fi

# ── 2. gnome-keyring-daemon (secrets component only) ─────────────────────────
# Only start if the secrets service isn't already registered on the bus.
if command -v gnome-keyring-daemon >/dev/null 2>&1; then
  if ! dbus-send --session --dest=org.freedesktop.DBus --type=method_call \
       --print-reply /org/freedesktop/DBus org.freedesktop.DBus.ListNames \
       2>/dev/null | grep -q "org.freedesktop.secrets"; then
    eval "$(gnome-keyring-daemon --start --components=secrets 2>/dev/null)" || true
  fi
fi

# ── 3. Hand off to copilot ───────────────────────────────────────────────────
# exec ensures signals (SIGTERM, SIGINT) propagate directly to copilot and the
# container exits when copilot does.
exec copilot "$@"

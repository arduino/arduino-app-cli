#!/bin/bash
# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

# Exposes a local port on the board's localhost, or closes it.
# BOARD unset: adb reverse. BOARD=user@host: background ssh -R, tracked by a
# control socket. BOARD_PASSWORD skips the prompt (via SSH_ASKPASS).
#   ./scripts/board-tunnel.sh open <port>
#   ./scripts/board-tunnel.sh close <port>

set -euo pipefail

if [ "$#" -ne 2 ] || { [ "$1" != "open" ] && [ "$1" != "close" ]; }; then
  echo "usage: $0 open|close <port>" >&2
  exit 1
fi

ACTION="$1"
PORT="$2"
SOCKET="$(cd "$(dirname "$0")/.." && pwd)/build/board-tunnel-${PORT}.sock"
LOG="${SOCKET%.sock}.log"

if [ -z "${BOARD:-}" ]; then
  if [ "$ACTION" = "open" ]; then
    adb reverse "tcp:$PORT" "tcp:$PORT"
  else
    adb reverse --remove "tcp:$PORT" 2>/dev/null || true
  fi
  exit 0
fi

# Close first: makes `open` idempotent.
if [ -S "$SOCKET" ]; then
  ssh -S "$SOCKET" -O exit "$BOARD" 2>/dev/null || true
  rm -f "$SOCKET"
fi

if [ "$ACTION" = "close" ]; then
  rm -f "$LOG"
  exit 0
fi

if [ -n "${BOARD_PASSWORD:-}" ]; then
  # Not sshpass: its pty hangup kills a ProxyCommand, dropping the tunnel.
  ASKPASS="$(mktemp)"
  trap 'rm -f "$ASKPASS"' EXIT
  printf '#!/bin/sh\necho "$BOARD_PASSWORD"\n' >"$ASKPASS"
  chmod 700 "$ASKPASS"
  export BOARD_PASSWORD SSH_ASKPASS="$ASKPASS" SSH_ASKPASS_REQUIRE=force
fi

mkdir -p "$(dirname "$SOCKET")"
# Output to a log: inherited stdout/stderr would keep pipes (`| tee`) open.
# ExitOnForwardFailure: fail if the port is taken on the board.
if ! ssh -f -N -M -S "$SOCKET" \
  -o ExitOnForwardFailure=yes \
  -R "$PORT:127.0.0.1:$PORT" \
  "$BOARD" >/dev/null 2>"$LOG"; then
  cat "$LOG" >&2
  exit 1
fi

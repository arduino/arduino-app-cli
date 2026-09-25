#!/bin/bash
# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

# Makes a port of this machine reachable from the board, as the same port on
# the board's localhost, or closes that tunnel.
#
# The transport is selected by the BOARD variable (it can be set in `.env.local`,
# which the Taskfile loads):
#   BOARD unset             -> adb reverse, for a board connected via USB
#   BOARD=arduino@<host>    -> ssh remote forwarding, kept in background
#
# Set BOARD_PASSWORD to avoid being asked for the board password; it is handed
# to ssh through SSH_ASKPASS, so it never shows up in the process list.
#
# The tunnel outlives the script, so that the board can keep reaching this
# machine after the task that opened it has returned. Over ssh, the background
# process is tracked through a control socket, which is how `close` finds it.
#
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

if [ -z "${BOARD:-}" ]; then
  if [ "$ACTION" = "open" ]; then
    adb reverse "tcp:$PORT" "tcp:$PORT"
  else
    adb reverse --remove "tcp:$PORT" 2>/dev/null || true
  fi
  exit 0
fi

# Closing first also makes `open` idempotent, replacing a tunnel left over by a
# previous run.
if [ -S "$SOCKET" ]; then
  ssh -S "$SOCKET" -O exit "$BOARD" 2>/dev/null || true
  rm -f "$SOCKET"
fi

if [ "$ACTION" = "close" ]; then
  exit 0
fi

if [ -n "${BOARD_PASSWORD:-}" ]; then
  # Not sshpass, unlike the other scripts: it runs ssh on a pseudo-terminal that
  # it closes as soon as ssh goes to background, and the hangup kills the
  # ProxyCommand, if the board is reached through one, taking the tunnel down.
  # The helper reads the password from the environment, so it holds no secret.
  ASKPASS="$(mktemp)"
  trap 'rm -f "$ASKPASS"' EXIT
  printf '#!/bin/sh\necho "$BOARD_PASSWORD"\n' >"$ASKPASS"
  chmod 700 "$ASKPASS"
  export BOARD_PASSWORD SSH_ASKPASS="$ASKPASS" SSH_ASKPASS_REQUIRE=force
fi

mkdir -p "$(dirname "$SOCKET")"
# The background process would inherit the caller's stdout and stderr and keep
# them open, so anything reading the task output (e.g. `| tee`) would never see
# it end. Its messages go to a log instead, shown if the tunnel does not open.
LOG="${SOCKET%.sock}.log"
# ExitOnForwardFailure makes the command fail if the port is already taken on
# the board, instead of leaving a tunnel that forwards nothing.
if ! ssh -f -N -M -S "$SOCKET" \
  -o ExitOnForwardFailure=yes \
  -R "$PORT:127.0.0.1:$PORT" \
  "$BOARD" >/dev/null 2>"$LOG"; then
  cat "$LOG" >&2
  exit 1
fi

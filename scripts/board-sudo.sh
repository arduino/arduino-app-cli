#!/bin/bash
# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

# Runs a command with sudo on the board.
# BOARD unset: adb. BOARD=user@host: ssh.
# BOARD_PASSWORD skips the prompts (ssh login via `sshpass -e`).
#   ./scripts/board-sudo.sh "<command>"

set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: $0 <command>" >&2
  exit 1
fi

# base64: survives local shell, remote shell and adb argument joining.
ENCODED_CMD="$(printf '%s' "$*" | base64 | tr -d '\n')"
# Decoded in a substitution: stdin stays free for `sudo -S`.
REMOTE_CMD="sudo -S sh -c \"\$(printf %s $ENCODED_CMD | base64 -d)\""

if [ -n "${BOARD_PASSWORD:-}" ]; then
  SUDO_PASS="$BOARD_PASSWORD"
else
  read -r -s -p "Enter device sudo password: " SUDO_PASS
  echo
fi

if [ -z "${BOARD:-}" ]; then
  echo "$SUDO_PASS" | adb shell "$REMOTE_CMD"
  exit 0
fi

if [ -n "${BOARD_PASSWORD:-}" ]; then
  if ! command -v sshpass >/dev/null; then
    echo "$0: BOARD_PASSWORD is set but sshpass is not installed" >&2
    exit 1
  fi
  export SSHPASS="$BOARD_PASSWORD"
  echo "$SUDO_PASS" | sshpass -e ssh "$BOARD" "$REMOTE_CMD"
else
  echo "$SUDO_PASS" | ssh "$BOARD" "$REMOTE_CMD"
fi

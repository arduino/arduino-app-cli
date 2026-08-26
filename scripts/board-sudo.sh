#!/bin/bash
# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

# Runs a command as sudo on the board, asking for the password interactively.
#
# The transport is selected by the BOARD variable (it can be set in `.env.local`,
# which the Taskfile loads):
#   BOARD unset             -> adb, for a board connected via USB
#   BOARD=arduino@<host>    -> ssh
#
# Set BOARD_PASSWORD to skip the prompt and run unattended; over ssh it is also
# handed to the login through `sshpass -e`, so it never shows up in the process
# list.
#
# The command is given as arguments, and may span multiple lines:
#   ./scripts/board-sudo.sh "apt-get update && apt-get install -y foo"

set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: $0 <command>" >&2
  exit 1
fi

# The command travels base64-encoded and is decoded on the board: that keeps it
# intact through the local shell, the remote shell, and (for adb) the way adb
# joins its arguments back into a single command line.
ENCODED_CMD="$(printf '%s' "$*" | base64 | tr -d '\n')"
# The decoding happens in a command substitution, so that stdin stays free for
# the password that `sudo -S` reads.
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

#!/bin/bash
# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

# Copies a local file or directory to the board.
#
# The transport is selected by the BOARD variable (it can be set in `.env.local`,
# which the Taskfile loads):
#   BOARD unset             -> adb, for a board connected via USB
#   BOARD=arduino@<host>    -> ssh/scp
#
# Set BOARD_PASSWORD to avoid being asked for the board password at every step;
# it is handed to ssh through `sshpass -e`, so it never shows up in the process
# list.
#
# The remote path is always the final destination, never the parent directory,
# and it is replaced if it already exists.

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <local-path> <remote-path>" >&2
  exit 1
fi

LOCAL_PATH="$1"
REMOTE_PATH="$2"

if [ -z "${BOARD:-}" ]; then
  adb shell "rm -rf '$REMOTE_PATH'"
  adb push "$LOCAL_PATH" "$REMOTE_PATH"
  exit 0
fi

if [ -n "${BOARD_PASSWORD:-}" ]; then
  if ! command -v sshpass >/dev/null; then
    echo "$0: BOARD_PASSWORD is set but sshpass is not installed" >&2
    exit 1
  fi
  export SSHPASS="$BOARD_PASSWORD"
  ssh_cmd() { sshpass -e ssh "$@"; }
  scp_cmd() { sshpass -e scp "$@"; }
else
  ssh_cmd() { ssh "$@"; }
  scp_cmd() { scp "$@"; }
fi

ssh_cmd "$BOARD" "rm -rf '$REMOTE_PATH'"
scp_cmd -r "$LOCAL_PATH" "$BOARD:$REMOTE_PATH"

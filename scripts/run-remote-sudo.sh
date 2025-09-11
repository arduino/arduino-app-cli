#!/bin/bash

# Runs a command as sudo on the board (adb shell), while prompting for the password interactively.
# To be used in the taskfile.

read -s -p "Enter device sudo password: " SUDO_PASS
echo

# Run the command remotely with sudo, piping the password
echo "$SUDO_PASS" | adb shell "sudo -S sh -c \"$*\""

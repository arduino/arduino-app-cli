#!/usr/bin/env bash
# Usage: source ./scripts/start-pipewire.sh

export XDG_RUNTIME_DIR=/run/user/$(id -u)
export DBUS_SESSION_BUS_ADDRESS=unix:path=${XDG_RUNTIME_DIR}/bus

sudo mkdir -p "$XDG_RUNTIME_DIR"
sudo chown "$(id -u):$(id -g)" "$XDG_RUNTIME_DIR"
sudo chmod 700 "$XDG_RUNTIME_DIR"

if ! pgrep -x dbus-daemon > /dev/null; then
    echo "Starting dbus..."
    dbus-daemon --session --address="$DBUS_SESSION_BUS_ADDRESS" --nofork &
fi

pkill -x pipewire pipewire-pulse wireplumber 2>/dev/null || true
sleep 0.5

pipewire & export PW_PID=$!
wireplumber & export WP_PID=$!
pipewire-pulse & export PP_PID=$!

sleep 1

pipewire_stop() {
    kill "${PW_PID:-}" "${WP_PID:-}" "${PP_PID:-}" 2>/dev/null || true
    echo "Pipewire stopped."
}

echo "Pipewire is running. Use 'pw-play <file>' to test audio."
echo "Run 'pipewire_stop' to shut down."

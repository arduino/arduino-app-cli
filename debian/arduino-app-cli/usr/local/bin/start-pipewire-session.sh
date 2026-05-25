#!/bin/bash
export XDG_RUNTIME_DIR=/run/user/1000
export DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus

# Start dbus
dbus-daemon --session --address="$DBUS_SESSION_BUS_ADDRESS" --nofork --nopidfile --syslog-only &

# Wait for socket
for i in $(seq 1 10); do
    [ -S /run/user/1000/bus ] && break
    sleep 0.5
done

# Start pipewire
pipewire &
wireplumber &

wait

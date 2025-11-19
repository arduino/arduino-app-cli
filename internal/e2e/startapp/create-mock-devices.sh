#!/bin/bash
set -e

# Mock GPU path
mkdir -p /dev/dri || true
if [ ! -e /dev/dri/renderD128 ]; then
    mknod /dev/dri/renderD128 c 226 128 || true
fi

# Mock camera
if [ ! -e /dev/video0 ]; then
    mknod /dev/video0 c 81 0 || true
fi

# Continue with the actual CMD
exec "$@"

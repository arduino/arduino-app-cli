#!/bin/sh

set -xe

# Set TMPDIR for all users
adb shell sh -c "cat > /etc/profile.d/50-tmpdir.sh" <<EOF
export TMPDIR=/tmp
EOF

adb shell apt-get update
adb shell apt-get upgrade -y
adb shell apt-get install -y tree curl htop zip unzip file kitty-terminfo gpiod


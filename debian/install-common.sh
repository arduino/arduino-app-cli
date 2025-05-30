#!/bin/sh

set -xe

adb shell apt-get update
adb shell apt-get install -y tree curl

# Set TMPDIR for all users
adb shell sh -c "cat > /etc/profile.d/50-tmpdir.sh" <<EOF
export TMPDIR=/tmp
EOF

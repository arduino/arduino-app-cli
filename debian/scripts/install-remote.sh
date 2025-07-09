#!/bin/sh

BASE_DIR=$(cd -- "$(dirname -- "$0")" && pwd)

set -xe

# Install avahi-daemon and openssh-server.
adb shell apt-get update
adb shell apt-get install -y avahi-daemon openssh-server
adb shell systemctl enable avahi-daemon
adb shell systemctl start avahi-daemon
adb shell systemctl enable ssh
adb shell systemctl start ssh

# Install avaihi service to be discovered by the cli.
adb push $BASE_DIR/arduino.service /etc/avahi/services/arduino.service
adb shell systemctl restart avahi-daemon

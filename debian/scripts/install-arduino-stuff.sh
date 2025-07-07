#!/bin/sh

set -xe

wget -N http://downloads.arduino.cc/keys/arduino.asc

adb push ./arduino.asc /etc/apt/keyrings/
adb shell chmod 644 /etc/apt/keyrings/arduino.asc
adb shell sh -c 'cat > /etc/apt/sources.list.d/arduino.list' <<EOF
deb [signed-by=/etc/apt/keyrings/arduino.asc] https://apt-repo.arduino.cc stable main
EOF

adb shell sh -c 'cat > /etc/apt/auth.conf.d/arduino.conf' <<EOF
machine apt-repo.arduino.cc
login arduino
password aptexperiment
EOF

adb shell apt-get update
adb shell apt-get install -y arduino-orchestrator arduino-router arduino-app-lab arduino-cli

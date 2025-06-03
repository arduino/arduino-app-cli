#!/bin/sh

adb shell sh -c 'cat > /etc/apt/sources.list.d/arduino.list' <<EOF
deb [trusted=yes] https://apt-repo.oniudra.cc stable main
EOF

adb shell sh -c 'cat > /etc/apt/auth.conf.d/arduino.conf' <<EOF
machine apt-repo.oniudra.cc
login arduino
password aptexperiment
EOF

adb shell apt-get update
adb shell apt-get install -y arduino-orchestrator arduino-router

#!/bin/sh

set -xe

# Install ArduinoCore-zephyr platform.
adb shell "install -o arduino -g arduino -d /home/arduino/.arduino15"
adb shell su - arduino -c "\"cat > /home/arduino/.arduino15/arduino-cli.yaml\"" <<EOF
network:
  connection_timeout: 1000s
EOF
adb shell su - arduino -c "\"arduino-cli core update-index --additional-urls=https://arduino:aptexperiment@apt-repo.oniudra.cc/zephyr-core-imola.json\""
adb shell su - arduino -c "\"arduino-cli core install arduino:zephyr --additional-urls=https://arduino:aptexperiment@apt-repo.oniudra.cc/zephyr-core-imola.json\""

# Flash zephyr bootloader in the microcontroller.
adb shell systemctl disable board-tests.service || true
adb shell su - arduino -c "\"arduino-cli burn-bootloader -b arduino:zephyr:unoq -P jlink\""


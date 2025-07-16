#!/bin/sh

BASE_DIR=$(cd -- "$(dirname -- "$0")" && pwd)

set -xe

# Install ArduinoCore-zephyr platform.
adb shell su - arduino -c "\"mkdir -p /home/arduino/.arduino15\""
adb shell su - arduino -c "\"cat > /home/arduino/.arduino15/arduino-cli.yaml\"" <<EOF
board_manager:
    additional_urls:
      - https://arduino:aptexperiment@apt-repo.oniudra.cc/zephyr-core-jomla.json
network:
  connection_timeout: 1000s
EOF
adb shell su - arduino -c "\"arduino-cli core install dev:zephyr\""

# Compile and install remoteocd.
GOARCH=arm64 GOOS=linux go build -o ./build/remoteocd ./cmd/remoteocd/
adb shell mkdir -p /home/arduino/.arduino15/packages/arduino/tools/remoteocd/0.0.1/
adb push ./build/remoteocd /home/arduino/.arduino15/packages/arduino/tools/remoteocd/0.0.1/remoteocd
rm ./build/remoteocd

# Flash zephyr bootloader in the microcontroller.
adb shell su - arduino -c "\"arduino-cli burn-bootloader -b dev:zephyr:jomla -P jlink\""

# Patch adbd-usb-gadget.
adb push $BASE_DIR/adbd-usb-gadget /usr/lib/android-sdk/platform-tools/adbd-usb-gadget
adb shell chmod +x /usr/lib/android-sdk/platform-tools/adbd-usb-gadget
adb shell systemctl restart adbd || true

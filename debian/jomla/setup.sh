#!/bin/sh

BASE_DIR=$(cd -- "$(dirname -- "$0")" && pwd)

set -xe

# Install Arduino CLI.
adb shell 'mkdir -p /opt/arduino-cli && curl -L https://downloads.arduino.cc/arduino-cli/arduino-cli_1.2.2_Linux_ARM64.tar.gz | tar -xz -C /opt/arduino-cli'
adb shell ln -s /opt/arduino-cli/arduino-cli /usr/local/bin/arduino-cli || true
adb shell 'su - arduino -c "arduino-cli core install arduino:zephyr --additional-urls https://downloads.arduino.cc/packages/package_zephyr_index.json"'

# Install ArduinoCore-zephyr platform.
adb push $BASE_DIR/ArduinoCore-zephyr.tar.xz /tmp/ArduinoCore-zephyr.tar.xz
adb shell tar -xJf /tmp/ArduinoCore-zephyr.tar.xz -C /opt/
adb shell mkdir -p /home/arduino/Arduino/hardware/dev
adb shell ln -s /opt/ArduinoCore-zephyr /home/arduino/Arduino/hardware/dev/zephyr || true
adb shell rm -f /tmp/ArduinoCore-zephyr.tar.xz

# Compile and install remoteocd.
GOARCH=arm64 GOOS=linux go build -o ./build/remoteocd ./cmd/remoteocd/
adb shell mkdir -p /home/arduino/.arduino15/packages/arduino/tools/remoteocd/0.0.1/
adb push ./build/remoteocd /home/arduino/.arduino15/packages/arduino/tools/remoteocd/0.0.1/remoteocd
rm ./build/remoteocd

# Flash zephyr bootloader in the microcontroller.
adb shell arduino-cli burn-bootloader -b dev:zephyr:jomla -P jlink

# Patch adbd-usb-gadget.
adb push $BASE_DIR/adbd-usb-gadget /usr/lib/android-sdk/platform-tools/adbd-usb-gadget
adb shell chmod +x /usr/lib/android-sdk/platform-tools/adbd-usb-gadget
adb shell systemctl restart adbd || true


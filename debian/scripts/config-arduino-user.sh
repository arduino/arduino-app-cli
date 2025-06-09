#!/bin/sh

set -xe

adb shell usermod -aG sudo arduino
adb shell usermod -aG netdev arduino
adb shell usermod -aG dialout arduino
adb shell usermod -aG docker arduino
adb shell usermod -aG video arduino
adb shell usermod -aG audio arduino

# Allow user to set manage networks
adb shell sh -c "cat > /etc/polkit-1/rules.d/50-sudo-networkmanager.rules" <<EOF
polkit.addRule(function(action, subject) {
  if (action.id.indexOf("org.freedesktop.NetworkManager") == 0 && subject.isInGroup("sudo") && subject.isInGroup("netdev")) {
    return polkit.Result.YES;
  }
});
EOF
adb shell systemctl restart polkit

# Allow user to set gpio
adb shell groupadd -f gpiod
adb shell usermod -aG gpiod arduino
adb shell sh -c  "cat > /usr/lib/udev/rules.d/60-gpiod.rules" <<EOF
SUBSYSTEM=="gpio", KERNEL=="gpiochip*", GROUP="gpiod", MODE="0660"
EOF


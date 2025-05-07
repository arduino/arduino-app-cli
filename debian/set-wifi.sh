#!/bin/sh

[ -z "$SSID" ] || [ -z "$PASSWORD" ] && {
  echo "SSID or PASSWORD not sets."
  exit 1
}

set -xe

adb shell sh -c 'cat > /etc/NetworkManager/system-connections/mywifi.nmconnection' <<EOF
[connection]
id=MyWiFi
uuid=12345678-1234-1234-1234-123456789abc
type=wifi
autoconnect=true

[wifi]
ssid=$SSID
mode=infrastructure

[wifi-security]
key-mgmt=wpa-psk
psk=$PASSWORD

[ipv4]
method=auto

[ipv6]
method=auto
EOF

adb shell chmod 600 /etc/NetworkManager/system-connections/mywifi.nmconnection

adb shell systemctl restart NetworkManager


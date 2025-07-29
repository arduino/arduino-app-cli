#!/bin/sh

[ -z "$SSID" ] || [ -z "$PASSWORD" ] && {
  echo "SSID or PASSWORD not sets."
  exit 1
}

set -xe

adb shell sh -c "cat > /etc/NetworkManager/system-connections/$SSID.nmconnection" <<EOF
[connection]
id=$SSID
uuid=$(uuidgen)
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

adb shell chmod 600 /etc/NetworkManager/system-connections/$SSID.nmconnection

adb shell systemctl restart NetworkManager

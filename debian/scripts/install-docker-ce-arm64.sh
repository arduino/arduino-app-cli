#!/bin/sh
set -xe

BASE_URL="https://download.docker.com/linux/debian/dists/trixie/pool/stable/arm64"

cd /tmp
wget -N "$BASE_URL/containerd.io_1.7.27-1_arm64.deb"
wget -N "$BASE_URL/docker-ce-cli_28.1.1-1~debian.13~trixie_arm64.deb"
wget -N "$BASE_URL/docker-ce_28.1.1-1~debian.13~trixie_arm64.deb"
wget -N "$BASE_URL/docker-compose-plugin_2.35.1-1~debian.13~trixie_arm64.deb"

adb push ./containerd.io_*_arm64.deb /tmp/
adb push ./docker-ce-cli_*_arm64.deb /tmp/
adb push ./docker-ce_*_arm64.deb /tmp/
adb push ./docker-compose-plugin_*_arm64.deb /tmp/

adb shell apt-get update
adb shell apt-get install -y /tmp/containerd.io_*_arm64.deb
adb shell apt-get install -y /tmp/docker-ce-cli_*_arm64.deb
adb shell apt-get install -y /tmp/docker-ce_*_arm64.deb
adb shell apt-get install -y /tmp/docker-compose-plugin_*_arm64.deb


# fix to use nftables instead iptableso
adb shell update-alternatives --set iptables /usr/sbin/iptables-legacy
adb shell update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy


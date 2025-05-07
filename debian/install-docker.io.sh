#!/bin/sh
set -xe

adb shell apt-get update
adb shell apt-get install -y docker.io docker-compose docker-cli docker-buildx-plugin

# fix to use nftables instead iptables
adb shell update-alternatives --set iptables /usr/sbin/iptables-legacy
adb shell update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy



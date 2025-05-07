#!/bin/sh

set -xe

adb shell apt-get update
adb shell apt-get install -y tree curl


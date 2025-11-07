#!/bin/sh
#
# Configure Avahi with the serial number.
# This operation is non-blocking: if it fails,
# the script will exit with success in order to 
# not to interrupt the post-install process.
#

TARGET_FILE="/etc/avahi/services/arduino.service"
MARKER_LINE="</service>"
SERIAL_NUMBER_PATH="/sys/devices/soc0/serial_number"

echo "Configuring Avahi with serial number for network discovery..."

if [ ! -f "$SERIAL_NUMBER_PATH" ]; then
    echo "Warning: Serial number path not found at $SERIAL_NUMBER_PATH. Skipping." >&2
    exit 0 
fi


if [ ! -w "$TARGET_FILE" ]; then
    echo "Warning: Target file $TARGET_FILE not found or not writable. Skipping." >&2
    exit 0
fi

SERIAL_NUMBER=$(cat "$SERIAL_NUMBER_PATH")

if [ -z "$SERIAL_NUMBER" ]; then
    echo "Warning: Serial number file is empty. Skipping." >&2
    exit 0 
fi

if grep -q "serial_number=${SERIAL_NUMBER}" "$TARGET_FILE"; then
    echo "Serial number ($SERIAL_NUMBER) already configured. Skipping."
    exit 0
fi

SERIAL_NUMBER_ESCAPED=$(echo "$SERIAL_NUMBER" | sed -e 's/\\/\\\\/g' -e 's/\//\\\//g' -e 's/\&/\\\&/g')
NEW_LINE="  <txt-record>serial_number=${SERIAL_NUMBER_ESCAPED}</txt-record>"

echo "Adding serial number to $TARGET_FILE..."

sed -i "\#${MARKER_LINE}#i ${NEW_LINE}" "$TARGET_FILE"

echo "Avahi configuration attempt finished."
exit 0
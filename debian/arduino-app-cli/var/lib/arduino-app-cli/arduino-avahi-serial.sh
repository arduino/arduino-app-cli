#!/bin/sh
# Configure Avahi with the serial number.


TARGET_FILE="/etc/avahi/services/arduino.service"
SERIAL_NUMBER_PATH="/sys/devices/soc0/serial_number"

echo "Configuring Avahi with serial number for network discovery..."

if [ ! -f "$SERIAL_NUMBER_PATH" ]; then
    echo "Error: Serial number path not found at $SERIAL_NUMBER_PATH." >&2
    exit 1 
fi


if [ ! -w "$TARGET_FILE" ]; then
    echo "Error: Target file $TARGET_FILE not found or not writable." >&2
    exit 1
fi

SERIAL_NUMBER=$(cat "$SERIAL_NUMBER_PATH")

if [ -z "$SERIAL_NUMBER" ]; then
    echo "Error: Serial number file is empty." >&2
    exit 1 
fi

if grep -q "serial_number=" "$TARGET_FILE"; then
    echo "Serial number ($SERIAL_NUMBER) already configured."
    exit 0
fi

echo "Adding serial number to $TARGET_FILE..."
sed -i "/<\/service>/i <txt-record>serial_number=${SERIAL_NUMBER}<\/txt-record>" "$TARGET_FILE"

echo "Avahi configuration attempt finished."
exit 0
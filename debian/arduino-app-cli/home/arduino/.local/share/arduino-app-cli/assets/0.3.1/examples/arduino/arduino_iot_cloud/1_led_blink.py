# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Arduino IoT Cloud LED Blink Example"
from arduino.app_bricks.arduino_iot_cloud import ArduinoIoTCloud
from arduino.app_utils import App
import time

# If secrets are not provided in the class initialization, they will be read from environment variables
iot_cloud = ArduinoIoTCloud()


def led_callback(client: object, value: bool):
    """Callback function to handle LED blink updates from cloud."""
    print(f"LED blink value updated from cloud: {value}")

iot_cloud.register("led", value=False, on_write=led_callback)

App.start_brick(iot_cloud)
while True:
    iot_cloud.led = not iot_cloud.led
    print(f"LED blink set to: {iot_cloud.led}")
    time.sleep(3)

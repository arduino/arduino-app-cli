# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Arduino Cloud Light with Colors Example"
from arduino.app_bricks.arduino_cloud import ArduinoCloud, ColoredLight
from arduino.app_utils import App
from typing import Any
import time
import random

# If secrets are not provided in the class initialization, they will be read from environment variables
arduino_cloud = ArduinoCloud()


def light_callback(client: object, value: Any):
    """Callback function to handle light updates from cloud."""
    print(f"Light value updated from cloud: {value}")


arduino_cloud.register(ColoredLight("clight", swi=True, on_write=light_callback))

App.start_brick(arduino_cloud)

while True:
    # randomize color
    arduino_cloud.clight.hue = random.randint(0, 360)
    arduino_cloud.clight.sat = random.randint(0, 100)
    arduino_cloud.clight.bri = random.randint(0, 100)
    print(f"Light set to hue: {arduino_cloud.clight.hue}, saturation: {arduino_cloud.clight.sat}, brightness: {arduino_cloud.clight.bri}")
    time.sleep(3)

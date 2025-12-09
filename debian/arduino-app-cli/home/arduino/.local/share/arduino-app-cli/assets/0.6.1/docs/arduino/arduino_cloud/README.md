# Arduino Cloud Brick

This Brick provides integration with the Arduino Cloud platform, enabling IoT devices to communicate and synchronize data seamlessly.

## Overview

The Arduino Cloud Brick simplifies the process of connecting your Arduino device to the Arduino Cloud. It abstracts the complexities of device management, authentication, and data synchronization, allowing developers to focus on building applications and features. With this module, you can easily register devices, exchange data, and leverage cloud-based automation for your projects.

## Features

- Connects Arduino devices to the Arduino Cloud
- Supports device registration and authentication
- Enables data exchange between devices and the cloud
- Provides APIs for sending and receiving data

## Prerequisites

To use this Brick, we need to have an active Arduino Cloud account, and a **device** and **thing** setup. To obtain the credentials, please follow the instructions at this [link](https://docs.arduino.cc/arduino-cloud/features/manual-device/). This is also covered in the [Blinking LED with Arduino Cloud](/examples/cloud-blink).

During the device configuration, we will obtain a `device_id` and `secret_key`, which is needed to use this Brick. Note that a Thing with the device associated is required, and that you will need to create variables / dashboard to send and receive data from the board.

### Adding Credentials

The `device_id` and `secret_key` can be added inside the Arduino Cloud brick, by clicking on the **Brick Configuration** button inside the Brick.

Clicking the button will provide two fields where the `device_id` and `secret_key` can be added to the Brick.

## Code Example and Usage

```python
from arduino.app_bricks.arduino_cloud import ArduinoCloud
from arduino.app_utils import App, Bridge

iot_cloud = ArduinoCloud()

def led_callback(client: object, value: bool):
    """Callback function to handle LED blink updates from cloud."""
    print(f"LED blink value updated from cloud: {value}")
    Bridge.call("set_led_state", value)

iot_cloud.register("led", value=False, on_write=led_callback)

App.run()
```
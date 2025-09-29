# Arduino Cloud Brick

This brick provides integration with the Arduino Cloud platform, enabling IoT devices to communicate and synchronize data seamlessly.

## Overview

The Arduino Cloud Brick simplifies the process of connecting your Arduino device to the Arduino Cloud. It abstracts the complexities of device management, authentication, and data synchronization, allowing developers to focus on building applications and features. With this module, you can easily register devices, exchange data, and leverage cloud-based automation for your projects.

## Features

- Connects Arduino devices to the Arduino Cloud
- Supports device registration and authentication
- Enables data exchange between devices and the cloud
- Provides APIs for sending and receiving data

## Prerequisites

To obtain your credentials, please follow the instructions at this [link](https://docs.arduino.cc/arduino-cloud/features/manual-device/)

Set your secret and device ID in your environment or pass them directly to the `ArduinoCloud` class.

## Code example and usage

```python
from appslab_arduino_cloud import ArduinoCloud

cloud = ArduinoCloud()
arduino_cloud.register("led", value=False, on_write=led_callback)
arduino_cloud.led = true
```

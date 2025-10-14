# Camera Code Detection Brick

This Brick enables real-time barcode and QR code scanning from a camera video stream. 

## Overview

The Camera Code Detection Brick allows you to:

- Capture frames from a USB camera.
- Configure camera settings (resolution and frame rate).
- Define the type of code to detect: barcodes and/or QR codes.
- Process detections with customizable callbacks.

## Features

- Supported Code Formats: 
  - **Linear**: EAN-13, EAN-8, UPC-A
  - **2D**: QR Code
- Single-code detection mode for focused scanning
- Multi-code detection for simultaneous barcode and QR code scanning
- Provides detection coordinates for precise code location

## Prerequisites

To use this Brick you should have a USB camera connected to your board.

**Tip**: Use a USB-C® Hub with USB-A connectors to support commercial web cameras.

## Code example and usage

```python
from arduino.app_bricks.camera_code_detection import CameraCodeDetection

def render_frame(frame):
    ...

def handle_detected_code(frame, detection):
    ...

# Select the camera you want to use, its resolution and the max fps
detection = CameraCodeDetection(camera=0, resolution=(640, 360), fps=10)
detection.on_frame(render_frame)
detection.on_detection(handle_detected_code)
detection.start()
```

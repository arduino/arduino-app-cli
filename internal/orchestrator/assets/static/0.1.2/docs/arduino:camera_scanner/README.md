# Camera code detector

This brick helps in acquiring a camera's video stream and scanning it for barcodes and QR codes.

## Features

- **Supported formats**: EAN-13, EAN-8, and UPC-A, and 2D QR codes;
- **Single code scanning**: detects a single code at a time;
- **Multiple code scanning**: detects a multiple codes at a time;
- **Mixed scanning**: by default, detects barcodes and QR codes simultaneously.

## How to use

```python
from arduino.app_bricks.camera_code_detector import CameraCodeDetector

def render_frame(frame):
    ...

def handle_detected_code(frame, detection):
    ...

# Select the camera you want to use, its resolution and the max fps
detector = CameraCodeDetector(camera=0, resolution=(640, 360), fps=10)
detector.on_frame(render_frame)
detector.on_detection(handle_detected_code)
detector.start()
```

# Video Object Detection Brick

This Brick provides a Python interface for **detecting objects in real time from a USB camera video stream**.  
It connects to a model runner over WebSocket, continuously analyzes incoming frames, and produces detection events with predicted labels, bounding boxes, and confidence scores.  

Beyond visualization, it allows you to **register callbacks** that react to detections, either for specific objects or for all detections, enabling event-driven logic in your applications.  
It supports both **pre-trained models** provided by the framework and **custom models** trained with Edge Impulse.

## Overview

The Video Object Detection Brick allows you to:

- Continuously detect objects from a live camera or video stream.
- Get bounding boxes, labels, and confidence scores in real time.
- Trigger custom Python functions when certain objects are detected.
- Handle all detections in a single callback if desired.
- Control confidence thresholds and debounce timing to avoid repeated triggers.
- Override the detection threshold dynamically at runtime (if supported by the model).

## Features

- Real-time detection stream with continuous object recognition.
- Outputs:
  - **Class label** (e.g., "person", "bicycle")
  - **Confidence score** for each detection
  - **Bounding boxes** for localized detections
- Two callback styles:
  - `on_detect("<label>", callback)` → React to a specific label.
  - `on_detect_all(callback)` → React to all detections at once.
- Configurable confidence threshold (default: `0.3`) and debounce time between repeated detections (default: `2.0s`)
- Runtime threshold override with `override_threshold(value)`
- Clean lifecycle control with `start()` / `stop()` and integration with `App.run()`.

## Prerequisites

To use this Brick you should have a USB camera connected to your board.

**Tip**: Use a USB-C® Hub with USB-A connectors to support commercial web cameras.

## Code example and usage

```python
from arduino.app_utils import App
from arduino.app_bricks.video_objectdetection import VideoObjectDetection

# Initialize detector with custom confidence and debounce settings
video_detector = VideoObjectDetection(confidence=0.4, debounce_sec=1.5)

# Callback when a "person" is detected (no arguments allowed)
def on_person_detected():
    print("🚨 Person detected in the video stream!")

video_detector.on_detect("person", on_person_detected)

# Callback for all detections (must take one dict argument)
def on_all_detections(detections: dict):
    # Example: {"person": 0.87, "bicycle": 0.66}
    print("All detections:", detections)

video_detector.on_detect_all(on_all_detections)

# Run the application (keeps the video detection loop active)
App.run()
```
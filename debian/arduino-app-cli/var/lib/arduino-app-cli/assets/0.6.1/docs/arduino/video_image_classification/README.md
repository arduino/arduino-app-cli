# Video Image Classification Brick

This Brick provides a Python® interface for **classifying video frames in real time** using a pre-trained machine learning model.

## Overview

The Video Image Classification Brick allows you to:

- Continuously analyze frames from a live video stream.
- Classify each frame's content into one or more categories.
- Receive real-time callbacks when specific labels are detected.
- React to *all* classifications through a consolidated callback.
- Use pre-trained models bundled with the framework or custom models trained on the **Edge Impulse** platform.

## Features

- Performs live classification on video streams.
- Outputs class labels with their associated confidence scores.
- Supports custom callback functions for specific labels.
- Provides a global callback to handle all classifications at once.
- Configurable **confidence threshold** and **debounce interval** to reduce noise and avoid repeated triggers.
- Easy integration with Python® applications.

## Prerequisites

To use this Brick you should have a USB camera connected to your board.

**Tip**: Use a USB-C® Hub with USB-A connectors to support commercial web cameras.

## Code example and usage

```python
from arduino.app_utils import App
from arduino.app_bricks.video_imageclassification import VideoImageClassification

# Create a classification stream with default confidence threshold (0.3)
classification_stream = VideoImageClassification()

# Example: callback when "sunglasses" are detected
def sunglass_detected():
    print("Detected sunglasses!")

classification_stream.on_detect("sunglasses", sunglass_detected)

# Example: callback for all classifications
def all_detected(results):
    print("Classifications:", results)

classification_stream.on_detect_all(all_detected)

App.run()
```

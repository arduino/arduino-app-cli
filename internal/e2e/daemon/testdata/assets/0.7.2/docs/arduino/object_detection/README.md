# Object Detection Brick

This Brick provides a Python interface for **detecting objects** within a given image.

## Overview

The Object Detection Brick allows you to:

- Detect objects in an image, either from a local file or directly from a camera feed.
- Locate detected objects in the image using bounding boxes.
- Get the detection confidence value of each object and its label.

## Features

- Performs real-time object detection on static images
- Outputs bounding boxes, class labels, and confidence scores for each detected object
- Supports multiple image formats, including JPEG, JPG, and PNG (default: JPG)
- Allows customization of detection confidence and non-maximum suppression (NMS) thresholds
- Easily integrates with PIL images or raw image byte streams

## Code example and usage

```python
import os
from arduino.app_bricks.object_detection import ObjectDetection
from arduino.app_utils.image import draw_bounding_boxes

object_detection = ObjectDetection()

# Image can be provided as bytes or PIL.Image
img = os.read("path/to/your/image.jpg")

out = object_detection.detect(img)
# You can also provide a confidence level
# out = object_detection.detect(frame, confidence = 0.35)
if out and "detection" in out:
    for i, obj_det in enumerate(out["detection"]):
        # For every object detected, print its details
        detected_object = obj_det.get("class_name", None)
        confidence = obj_det.get("confidence", None)
        bounding_box = obj_det.get("bounding_box_xyxy", None)

# Draw the bounding boxes
out_image = draw_bounding_boxes(img, out)
```


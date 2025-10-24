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

object_detection = ObjectDetection()

# Image frame can be as bytes or PIL image
frame = os.read("path/to/your/image.jpg")

out = object_detection.detect(frame)
# is it possible to customize image type, confidence level and box overlap
# out = object_detection.detect(frame, image_type = "png", confidence = 0.35, overlap = 0.5)
if out and "detection" in out:
    for i, obj_det in enumerate(out["detection"]):
        # For every object detected, get its details
        detected_object = obj_det.get("class_name", None)
        bounding_box = obj_det.get("bounding_box_xyxy", None)
        confidence = obj_det.get("confidence", None)

# draw the bounding box and key points on the image
out_image = object_detection.draw_bounding_boxes(frame, out)
```


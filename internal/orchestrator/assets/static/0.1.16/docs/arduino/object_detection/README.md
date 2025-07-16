# Object detection

Detects objects within an image, returning bounding boxes, class labels, and confidence scores. Uses YOLO v11 as the default model.

## Features

- Real-time object detection on images
- Returns bounding boxes, class labels, and confidence scores
- Supports multiple image formats (JPEG, PNG, etc.)
- Adjustable detection confidence and box overlap thresholds
- Easy integration with PIL images or raw bytes

## Code example and usage

```python
import os
from arduino.app_bricks.objectdetection import ObjectDetection

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

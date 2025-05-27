# Image classification brick

Classifies image content by detecting objects and their confidence scores. Uses YOLO v11 as the default model.

## Code example and usage

```python
import os
from arduino.app_bricks.imageclassification import ImageClassification

image_classification = ImageClassification()

# Image frame can be as bytes or PIL image
frame = os.read("path/to/your/image.jpg")

out = image_classification.classify(frame)
# is it possible to customize image type and confidence level
# out = image_classification.classify(frame, image_type = "png", confidence = 0.35)
if out and "detection" in out:
    for i, obj_det in enumerate(out["detection"]):
        # For every object detected, get its details
        detected_object = obj_det.get("class_name", None)
        confidence = obj_det.get("confidence", None)
```

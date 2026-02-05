# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Object Detection"
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

# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Object Detection"
from arduino.app_bricks.object_detection import ObjectDetection

object_detection = ObjectDetection()

# Image frame can be as bytes or PIL image
with open("image.png", "rb") as f:
    frame = f.read()

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

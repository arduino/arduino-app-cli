# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Image classification"
from arduino.app_bricks.image_classification import ImageClassification

image_classification = ImageClassification()

# Image frame can be as bytes or PIL image
with open("image.jpg", "rb") as f:
    frame = f.read()

out = image_classification.classify(frame)
# is it possible to customize image type and confidence level
# out = image_classification.classify(frame, image_type = "png", confidence = 0.35)
if out and "classification" in out:
    for i, obj_det in enumerate(out["classification"]):
        # For every object detected, get its details
        detected_object = obj_det.get("class_name", None)
        confidence = obj_det.get("confidence", None)

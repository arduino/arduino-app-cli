# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Visual Anomaly Detection"
import os
from arduino.app_bricks.visual_anomaly_detection import VisualAnomalyDetection
from arduino.app_utils.image import draw_anomaly_markers

anomaly_detection = VisualAnomalyDetection()

# Image can be provided as bytes or PIL.Image
img = os.read("path/to/your/image.jpg")

out = anomaly_detection.detect(img)
if out and "detection" in out:
    for i, anomaly in enumerate(out["detection"]):
        # For every anomaly detected, print its details
        detected_anomaly = anomaly.get("class_name", None)
        score = anomaly.get("score", None)
        bounding_box = anomaly.get("bounding_box_xyxy", None)

# Draw the bounding boxes
out_image = draw_anomaly_markers(img, out)

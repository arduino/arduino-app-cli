# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Code detection (aggregated result)"
# EXAMPLE_REQUIRES = "Requires an USB webcam connected to the Arduino board."
from PIL.Image import Image
from arduino.app_utils import App
from arduino.app_bricks.camera_code_detection import CameraCodeDetection, Detection


def on_codes_detected(frame: Image, detections: list[Detection]):
    """Callback function that handles multiple detected codes."""
    print(f"Detected {len(detections)} codes")
    # Here you can add your code to process the detected codes,
    # e.g., draw bounding boxes, save them to a database or log them.


detector = CameraCodeDetection()
detector.on_detect(on_codes_detected)

App.run()  # This will block until the app is stopped

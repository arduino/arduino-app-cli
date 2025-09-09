# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Code detection"
# EXAMPLE_REQUIRES = "Requires an USB webcam connected to the Arduino board."
from PIL.Image import Image
from arduino.app_utils import App
from arduino.app_bricks.camera_code_detection import CameraCodeDetection, Detection


def on_code_detected(frame: Image, detection: Detection):
    """Callback function that handles a detected code."""
    print(f"Detected {detection.type} with content: {detection.content}")
    # Here you can add your code to process the detected code,
    # e.g., draw a bounding box, save it to a database or log it.


detector = CameraCodeDetection()
detector.on_detect(on_code_detected)

App.run()

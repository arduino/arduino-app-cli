# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Get user confirmation with hand gestures"
# EXAMPLE_REQUIRES = "Requires a connected camera"

from arduino.app_bricks.gesture_recognition import GestureRecognition
from arduino.app_utils.app import App

pd = GestureRecognition()
pd.on_gesture("Thumb_Up", lambda meta: print("Operation confirmed!"))
pd.on_gesture("Thumb_Down", lambda meta: print("Operation denied!"))

App.run()

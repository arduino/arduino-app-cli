# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Gesture recognition"
# EXAMPLE_REQUIRES = "Requires a connected camera"

from arduino.app_bricks.gesture_recognition import GestureRecognition
from arduino.app_utils.app import App

pd = GestureRecognition()
pd.on_gesture("Victory", lambda meta: print("All your bases are belong to us"))
pd.on_gesture("Open_Palm", lambda meta: print("Moving left!"), hand="left")
pd.on_gesture("Open_Palm", lambda meta: print("Moving right!"), hand="right")

App.run()

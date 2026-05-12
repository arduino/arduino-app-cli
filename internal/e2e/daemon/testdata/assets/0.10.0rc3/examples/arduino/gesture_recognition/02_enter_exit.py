# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Gesture recognition hand enter/exit detection"
# EXAMPLE_REQUIRES = "Requires a connected camera"

from arduino.app_bricks.gesture_recognition import GestureRecognition
from arduino.app_utils.app import App

pd = GestureRecognition()
pd.on_enter(lambda: print("Hi there!"))
pd.on_exit(lambda: print("Goodbye!"))

App.run()

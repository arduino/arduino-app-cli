# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect the glass breaking sound from microphone"
# EXAMPLE_REQUIRES = "Requires an USB microphone connected to the Arduino board."
from arduino.app_bricks.audio_classification import AudioClassification
from arduino.app_utils import App

classifier = AudioClassification()
classifier.on_detect("glass_breaking", lambda: print(f"Glass breaking sound detected!"))

App.run()

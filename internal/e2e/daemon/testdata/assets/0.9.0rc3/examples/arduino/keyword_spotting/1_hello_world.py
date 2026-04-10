# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect the 'hello world' keyword"
# EXAMPLE_REQUIRES = "Requires an USB microphone connected to the Arduino board."
from arduino.app_bricks.keyword_spotting import KeywordSpotting
from arduino.app_utils import App


spotter = KeywordSpotting()
spotter.on_detect("helloworld", lambda: print(f"Hello world detected!"))

App.run()

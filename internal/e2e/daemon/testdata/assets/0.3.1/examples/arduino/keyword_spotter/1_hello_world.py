# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect the 'hello world' keyword"
# EXAMPLE_REQUIRES = "Requires an USB microphone connected to the Arduino board."
from arduino.app_bricks.keyword_spotter import KeywordSpotter
from arduino.app_utils import App


spotter = KeywordSpotter()
spotter.on_detect("helloworld", lambda: print(f"Hello world detected!"))

App.run()

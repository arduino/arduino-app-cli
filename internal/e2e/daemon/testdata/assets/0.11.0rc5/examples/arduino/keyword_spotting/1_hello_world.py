# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect the 'hello world' keyword"
# EXAMPLE_REQUIRES = "Requires an USB microphone connected to the Arduino board."
from arduino.app_bricks.keyword_spotting import KeywordSpotting
from arduino.app_utils import App


spotting = KeywordSpotting()
spotting.on_detect("helloworld", lambda: print(f"Hello world detected!"))

App.run()

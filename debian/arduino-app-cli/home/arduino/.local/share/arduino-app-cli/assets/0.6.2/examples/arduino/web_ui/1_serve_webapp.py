# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Serve a web application"
# EXAMPLE_REQUIRES = "Requires an 'assets' directory in the app's root folder with an index.html file."
from arduino.app_utils import App
from arduino.app_bricks.web_ui import WebUI


WebUI()

App.run()  # This will block until the app is stopped

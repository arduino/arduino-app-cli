# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Listen for messages from connected clients"
from arduino.app_utils import App
from arduino.app_bricks.web_ui import WebUI


ui = WebUI()
ui.on_message("hello", lambda data: print(f"Received message: {data}"))

App.run()  # This will block until the app is stopped

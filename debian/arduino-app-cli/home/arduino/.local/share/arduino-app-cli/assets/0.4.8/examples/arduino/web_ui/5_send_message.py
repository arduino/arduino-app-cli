# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Send a message to a client that connects"
from arduino.app_utils import App
from arduino.app_bricks.web_ui import WebUI


ui = WebUI()
ui.on_connect(lambda sid: ui.send_message("hello", {"message": f"Hello, {sid}!"}))

App.run()  # This will block until the app is stopped

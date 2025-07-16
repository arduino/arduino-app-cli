# Web UI

This brick allows to host an HTML+JavaScript web application and/or APIs for clients to call.
By default, a web server will be hosted on port 7000 (default).

## Features

- Serves static HTML, CSS, and JavaScript files
- Supports RESTful API endpoints
- Customizable routes and handlers
- Simple configuration for port and root directory
- Lightweight and suitable for embedded devices
- Logging of HTTP requests and errors

## Code example and usage

```python
from app_bricks.web_ui import WebUI

# Initialize the Web UI server
web_ui = WebUI()

# Add a simple REST API endpoint
web_ui.expose_api("GET", "/hello", lambda: {"message": "Hello, world!"})

# Send a message to clients over WebSocket
web_ui.send_message("hello", {"message": "Hello!"})

# Start the server
web_ui.start()

# The server will now serve static files and respond to /api/hello requests
```


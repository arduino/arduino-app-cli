# Web UI Brick

This Brick is a simplified, embeddable web server designed for hosting frontend applications and exposing APIs or WebSocket communication channels.

## Overview

The Web UI Brick allows you to:

- Serve an HTML+JavaScript web interface (e.g., dashboards, control panels, SPAs)
- Expose REST APIs to be consumed by your frontend or third-party clients
- Communicate in real time with browsers using WebSockets
- Integrate with other bricks to display data or control devices over the network

Once started, your application will be accessible via a web browser at `http://<device-ip>:<port>` (default port 7000).

## Features

- Serves static HTML, CSS, and JavaScript files
- Supports RESTful API endpoints using FastAPI-style handlers
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

## Project Folder Structure

To use the Web UI Brick effectively, organize your project with the following structure:

```
your_project/
├── assets/                       # Static frontend files served at http://<host>:<port>/
│   ├── index.html                # 🔹 Entry point for your web UI (required)
│   ├── app.js                    # JavaScript logic for client-side interaction
│   └── style.css                 # Optional CSS for styling the UI
│
├── python/                       # Python backend code
│   └── main.py                   # 🔹 Your app’s main script that starts the WebUI server
│
├── app.yaml                      # Your app metadata
         
```


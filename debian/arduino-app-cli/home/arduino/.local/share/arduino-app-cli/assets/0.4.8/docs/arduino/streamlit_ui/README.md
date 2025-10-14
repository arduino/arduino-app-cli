# WebUI - Streamlit Brick

This brick enables you to create and host interactive, Python-based web applications powered by the **Streamlit** framework.

## Overview

The WebUI - Streamlit Brick allows you to:

- Build rich, interactive UIs using simple Python syntax
- Display real-time data from sensors, devices, or external APIs
- Trigger actions in other bricks or microcontrollers through buttons, sliders, or inputs

When running, your application will be accessible via a web browser at `http://<device-ip>:7000`

## Features

- Enables Streamlit web server functionality on port 7000
- Supports interactive UI components for data visualization and input
- Easily integrates with other Python modules and Arduino bricks
- Supports themes, layout customization, and Markdown/HTML rendering

## Code example and usage

```python
from arduino.app_bricks.streamlit_ui import st

st.title("Arduino Streamlit UI Example")
st.write("Interact with your Arduino modules using this web interface.")

if st.button("Send Command"):
    st.success("Command sent to Arduino!")
    
```


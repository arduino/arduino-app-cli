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

## How `App.run()` works with Streamlit

The Streamlit brick is executed via `streamlit run`, which manages its own event loop and runs your script in a worker thread. `AppController` automatically detects this environment and adapts its behavior:

- Bricks are started normally (their daemon threads run in the background)
- The blocking loop is skipped (Streamlit owns the process lifecycle)
- `App.run()` is idempotent: Streamlit re-runs the script on every user interaction, but bricks are only started once

### Brick instantiation with `@st.cache_resource`

Streamlit re-executes the entire script on every user interaction. This means any brick instantiated at top-level will be **re-created** on every re-run, potentially reopening hardware resources (e.g. ALSA devices, cameras, network connections).

To avoid this, wrap brick instantiation in `@st.cache_resource`:

```python
from arduino.app_utils import App
from arduino.app_bricks.streamlit_ui import st
from arduino.app_bricks.sound_generator import SoundGenerator, SoundEffect


@st.cache_resource
def init_bricks():
    return SoundGenerator(sound_effects=[SoundEffect.adsr()])


player = init_bricks()

st.title("My App")

if st.button("Play"):
    player.play("C4", 1.0)

App.run()
```

`App.run()` does not need to be cached — it is already idempotent and returns immediately on subsequent re-runs.

See the `examples/` folder for complete examples.

## Code example and usage

```python
from arduino.app_bricks.streamlit_ui import st

st.title("Arduino Streamlit UI Example")
st.write("Interact with your Arduino modules using this web interface.")

if st.button("Send Command"):
    st.success("Command sent to Arduino!")
    
```


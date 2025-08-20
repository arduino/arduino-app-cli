# Streamlit UI

This brick allows to host a Python-based web application powered by the Streamlit framework.
UI will be available on port 7000.

## Features

- Launches a Streamlit web server on port 7000
- Supports interactive UI components for data visualization and input
- Easily integrates with other Python modules and Arduino bricks
- Customizable layout and theming options

## Code example and usage

```python
from arduino.app_bricks.streamlit_ui import st

st.title("Arduino Streamlit UI Example")
st.write("Interact with your Arduino modules using this web interface.")

if st.button("Send Command"):
    st.success("Command sent to Arduino!")
    
```


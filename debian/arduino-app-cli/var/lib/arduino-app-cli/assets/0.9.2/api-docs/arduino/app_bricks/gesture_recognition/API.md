# gesture_recognition API Reference

## Index

- Class `GestureRecognition`

---

## `GestureRecognition` class

```python
class GestureRecognition(camera: BaseCamera | None, confidence: float)
```

### Methods

#### `start()`

Start the capture thread and asyncio event loop.

#### `stop()`

Stop all tracking and close connections.

#### `on_gesture(gesture: str, callback: Callable[[dict], None], hand: Literal['left', 'right', 'both'])`

Register or unregister a gesture callback.

##### Parameters

- **gesture** (*str*): The gesture name to detect
- **callback** (*Callable[[dict], None]*): Function to call when gesture is detected. None to unregister.
The callback receives a metadata dictionary with details about the detection, including:
- "hand": Which hand performed the gesture ("left" or "right")
- "gesture": Name of the detected gesture
- "confidence": Confidence score of the detection (0.0 to 1.0)
- "landmarks": List of key points of the detected hand (in (x, y, z) format where
    x and y are pixel coordinates and z is normalized depth)
- "bounding_box_xyxy": [x_min, y_min, x_max, y_max] of the detected hand bounding box
- **hand** (*Literal["left", "right", "both"]*): Which hand(s) to track

##### Raises

- **ValueError**: If 'hand' argument is not valid

#### `on_enter(callback: Callable[[], None])`

Register a callback for when hands become visible.

##### Parameters

- **callback** (*Callable[[], None]*): Function to call when at least one hand is detected

#### `on_exit(callback: Callable[[], None])`

Register a callback for when hands are no longer visible.

##### Parameters

- **callback** (*Callable[[], None]*): Function to call when no hands are detected anymore

#### `on_frame(callback: Callable[[np.ndarray], None])`

Register a callback that receives each camera frame.

##### Parameters

- **callback** (*Callable[[np.ndarray], None]*): Function to call with camera frame data. None to unregister.


# video_object_detection API Reference

## Index

- Class `VideoObjectDetection`

---

## `VideoObjectDetection` class

```python
class VideoObjectDetection(camera: BaseCamera | None, confidence: float, debounce_sec: float)
```

Module for object detection on a **live video stream** using a specified machine learning model.

This brick:
  - Connects to a model runner over WebSocket.
  - Parses incoming classification messages with bounding boxes.
  - Filters detections by a configurable confidence threshold.
  - Debounces repeated triggers of the same label.
  - Invokes per-label callbacks and/or a catch-all callback.

### Parameters

- **camera** (*BaseCamera*): The camera instance to use for capturing video. If None, a default camera will be initialized.
- **confidence** (*float*): Confidence level for detection. Default is 0.3 (30%).
- **debounce_sec** (*float*): Minimum seconds between repeated detections of the same object. Default is 0 seconds.

### Raises

- **RuntimeError**: If the host address could not be resolved.

### Methods

#### `on_detect(object: str, callback: Callable[[], None])`

Register a callback invoked when a **specific label** is detected.

##### Parameters

- **object** (*str*): The label of the object to check for in the classification results.
- **callback** (*Callable[[], None]*): A function with **no parameters**.

##### Raises

- **TypeError**: If `callback` is not a function.
- **ValueError**: If `callback` accepts any parameters.

#### `on_detect_all(callback: Callable[[dict], None])`

Register a callback invoked for **every detection event**.

This is useful to receive a consolidated dictionary of detections for each frame.

##### Parameters

- **callback** (*Callable[[dict], None]*): A function that accepts **one dict argument** with
the shape `{label: confidence, ...}`.

##### Raises

- **TypeError**: If `callback` is not a function.
- **ValueError**: If `callback` does not accept exactly one argument.

#### `start()`

Start the video object detection process.

#### `stop()`

Stop the video object detection process and release resources.

#### `object_detection_loop()`

Object detection main loop.

Maintains WebSocket connection to the model runner and processes object detection messages.
Retries on connection errors until stopped.

#### `camera_loop()`

Camera main loop.

Captures images from the camera and forwards them over the TCP connection.
Retries on connection errors until stopped.

#### `override_threshold(value: float)`

Override the threshold for object detection model.

##### Parameters

- **value** (*float*): The new value for the threshold in the range [0.0, 1.0].

##### Raises

- **TypeError**: If the value is not a number.
- **RuntimeError**: If the model information is not available or does not support threshold override.


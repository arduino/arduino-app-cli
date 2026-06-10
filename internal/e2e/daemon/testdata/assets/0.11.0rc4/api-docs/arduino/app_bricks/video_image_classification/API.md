# video_image_classification API Reference

## Index

- Class `VideoImageClassification`

---

## `VideoImageClassification` class

```python
class VideoImageClassification(camera: BaseCamera | None, confidence: float, debounce_sec: float)
```

Module for image classification on a **live video stream** using a specified machine learning model.

Provides a way to react to detected classes over a video stream invoking registered actions in real-time.

### Parameters

- **camera** (*BaseCamera*): The camera instance to use for capturing video. If None, a default camera will be initialized.
- **confidence** (*float*): The minimum confidence level for a classification to be considered valid. Default is 0.3.
- **debounce_sec** (*float*): The minimum time in seconds between consecutive detections of the same object
to avoid multiple triggers. Default is 0 seconds.

### Raises

- **RuntimeError**: If the host address could not be resolved.

### Methods

#### `on_detect_all(callback: Callable[[dict], None])`

Register a callback invoked for **every classification event**.

This callback is useful if you want to process all classified labels in a single
place, or be notified about any classification regardless of its type.

##### Parameters

- **callback** (*Callable[[dict], None]*): A function that accepts **exactly one argument**: a dictionary of
classifications above the confidence threshold, in the form
``{"label": confidence, ...}``.

##### Raises

- **TypeError**: If `callback` is not a function.
- **ValueError**: If `callback` does not accept exactly one argument.

#### `on_detect(object: str, callback: Callable[[], None])`

Register a callback invoked when a **specific label** is classified.

The callback is triggered whenever the given label appears in the classification
results and passes the confidence and debounce filters.

##### Parameters

- **object** (*str*): The label to listen for (e.g., ``"dog"``).
- **callback** (*Callable[[], None]*): A function with **no parameters** that will be executed when the
label is detected.

##### Raises

- **TypeError**: If `callback` is not a function.
- **ValueError**: If `callback` accepts one or more parameters.

#### `start()`

Start the classification.

#### `stop()`

Stop the classification and release resources.

#### `classification_loop()`

Classification main loop.

Maintains WebSocket connection to the model runner and processes classification messages.
Retries on connection errors until stopped.

#### `camera_loop()`

Camera main loop.

Captures images from the camera and forwards them over the TCP connection.
Retries on connection errors until stopped.

#### `override_threshold(value: float)`

Override the threshold for image classification model.

##### Parameters

- **value** (*float*): The new value for the threshold.

##### Raises

- **TypeError**: If the value is not a number.
- **RuntimeError**: If the model information is not available or does not support threshold override.


# video_objectdetection API Reference

## Index

- Class `VideoObjectDetection`

---

## `VideoObjectDetection` class

```python
class VideoObjectDetection(confidence: float, debounce_sec: float)
```

VideoObjectDetection module for detecting objects on video stream using a specified model.

Provides a way to react on detected objects over a video stream invoking registered actions in realtime.

### Parameters

- **confidence** (*float*): Confidence level for detection. Default is 0.3 (30%).
- **debounce_sec** (*float*): Minimum seconds between repeated detections of the same object. Default is 2.0 seconds.

### Raises

- **RuntimeError**: If the host address could not be resolved.

### Methods

#### `on_detect(object: str, callback: Callable[[], None])`

Register a callback function to be invoked when a specific object is detected.

##### Parameters

- **object** (*str*): The object to check for in the classification results.
- **callback** (*callable*): a callback function to handle the keyword spotted.

#### `on_detect_all(callback: Callable[[dict], None])`

Register a callback function to be invoked for all detected objects.

This function is useful when you want to handle all detected objects in a single callback or
if you want to be notified about any detected object regardless of its type.

##### Parameters

- **callback** (*callable*): a callback function to handle the detected object. It must accept one argument which is a dictionary containing the detected object information.

#### `start()`

Start the video object detection process.

#### `stop()`

Stop the video object detection process.

#### `override_threshold(value: float)`

Override the threshold for object detection model.

##### Parameters

- **value** (*float*): The new value for the threshold.

##### Raises

- **TypeError**: If the value is not a number.
- **RuntimeError**: If the model information is not available or does not support threshold override.


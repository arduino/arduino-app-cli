# video_imageclassification API Reference

## Index

- Class `VideoImageClassification`

---

## `VideoImageClassification` class

```python
class VideoImageClassification(confidence: float, debounce_sec: float)
```

VideoImageClassification module for classifying images on video stream using a specified model.

Provides a way to react on detected objects over a video stream invoking registered actions in realtime.

### Parameters

- **confidence** (*float*): The minimum confidence level for a classification to be considered valid. Default is 0.3.
- **debounce_sec** (*float*): The minimum time in seconds between consecutive detections of the same object to avoid multiple triggers. Default is 2.0 seconds.

### Raises

- **RuntimeError**: If the host address could not be resolved.

### Methods

#### `on_detect_all(callback: Callable[[dict], None])`

Register a callback function to be invoked for all classified objects.

This function is useful when you want to handle all classified objects in a single callback or
if you want to be notified about any classified object regardless of its type.

##### Parameters

- **callback** (*callable*): a callback function to handle the classification. It must accept one argument which is a dictionary containing the classified object information.

#### `on_detect(object: str, callback: Callable[[], None])`

Register a callback function to be invoked when a specific object is classified.

##### Parameters

- **object** (*str*): The object to check for in the classification results.
- **callback** (*callable*): a callback function to handle the keyword spotted.

#### `start()`

Start the VideoImageClassification module and begin processing video classification.

#### `stop()`

Stop the VideoImageClassification module and release resources.

#### `execute()`

Main execution loop for the VideoImageClassification module.

Connects to the WebSocket server and processes incoming classification messages.


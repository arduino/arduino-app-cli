# motion_detection API Reference

## Index

- Class `MotionDetection`

---

## `MotionDetection` class

```python
class MotionDetection(confidence: float)
```

This Motion Detection module classifies motion patterns using accelerometer data.

### Parameters

- **confidence** (*float*): Confidence level for detection. Default is 0.4 (40%).

### Methods

#### `MotionDetection.accumulate_samples(accelerometer_samples: Tuple[float, float, float])`

Accumulate accelerometer samples for motion detection.

##### Parameters

- **accelerometer_samples** (*tuple*): A tuple containing x, y, z acceleration values. Typically, these values are in m/s^2, but depends on the model configuration.

#### `MotionDetection.on_movement_detection(keyword: str, callback: callable)`

Classify an audio file to detect keywords, and invoke a callback if the specified keyword is spotted.

##### Parameters

- **keyword** (*str*): The keyword to check for in the classification results.
- **callback** (*callable*): a callback function to handle the keyword spotted.

##### Raises

- **ValueError**: If the sample width of the audio file is unsupported.

#### `MotionDetection.get_sensor_samples()`

Get the current sensor samples.

##### Returns

- (*iterable*): An iterable containing the accumulated sensor data (x, y, z acceleration values).

#### `MotionDetection.start()`

Start the MotionDetection module and prepare for motion detection.

#### `MotionDetection.stop()`

Stop the MotionDetection module and release resources.


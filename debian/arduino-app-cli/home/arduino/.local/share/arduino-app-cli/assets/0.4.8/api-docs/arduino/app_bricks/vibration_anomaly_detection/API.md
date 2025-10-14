# vibration_anomaly_detection API Reference

## Index

- Class `VibrationAnomalyDetection`

---

## `VibrationAnomalyDetection` class

```python
class VibrationAnomalyDetection(anomaly_detection_threshold: float)
```

This Anomaly Detection module classifies accelerometr sensor data to detect vibration anomalies based on a pre-trained model.

### Parameters

- **anomaly_detection_threshold** (*float*): Confidence level for the anomaly score. Default is 1 (absolute value).

### Methods

#### `accumulate_samples(sensor_samples: Iterable[float])`

Accumulate sensor samples

##### Parameters

- **sensor_samples** (*Iterable*): An iterable of sensor samples (e.g., accelerometer data).

#### `on_anomaly(callback: callable)`

Register a callback function to be invoked when an anomaly is detected.

Function signature of the callback should be:
       - callback()  # No parameters
       - callback(anomaly_score: float)
       - callback(anomaly_score: float, classification: dict)

##### Parameters

- **callback** (*callable*): a callback function to handle label spotted.

##### Raises

- **ValueError**: If the sample width is unsupported.

#### `loop()`

Main loop for anomaly detection, processing sensor data and invoking callbacks when anomalies are detected.

#### `start()`

Start the AnomalyDetection module and prepare for motion detection.

#### `stop()`

Stop the AnomalyDetection module and release resources.


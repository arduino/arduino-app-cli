# visual_anomaly_detection API Reference

## Index

- Class `VisualAnomalyDetection`

---

## `VisualAnomalyDetection` class

```python
class VisualAnomalyDetection()
```

VisualAnomalyDetection module for detecting anomalies in images using a specified model.

### Methods

#### `detect_from_file(image_path: str)`

Process an image to detect anomalies.

##### Parameters

- **image_path**: fs path of the image to process.

#### `detect(image_bytes, image_type: str)`

Process an image to detect anomalies.

##### Parameters

- **image_bytes**: can be raw bytes or PIL image.
- **image_type**: type of image (jpg, jpeg, png). Default is jpg.

#### `process(item)`

Process an item to detect objects in an image.

##### Parameters

- **item**: A file path (str) or a dictionary with the 'image' and 'image_type' keys (dict).
'image_type' is optional while 'image' contains image as bytes.


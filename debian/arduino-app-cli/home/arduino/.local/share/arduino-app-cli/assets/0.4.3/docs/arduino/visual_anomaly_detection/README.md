# Visual Anomaly Detection Brick

This module enables detection of unusual patterns or defects in images, making it ideal for quality control, monitoring, and automation tasks in Arduino projects.

## Overview

The Visual Anomaly Detection Brick provides a modular component for detecting visual anomalies in images using machine learning techniques. It is designed to be easily integrated into Arduino-based applications.

## Features

- Image preprocessing and normalization
- Anomaly detection using pre-trained models
- Configurable detection thresholds
- Simple API for integration

## Code example and usage

```python
from app_bricks.visual_anomaly_detection import VisualAnomalyDetector

detector = VisualAnomalyDetector(model_path="path/to/model")
out = detector.detect(image)

if out and "detection" in out:
    for i, obj_det in enumerate(out["detection"]):
        # For every object detected, get its details
        detected_object = obj_det.get("class_name", None)
        bounding_box = obj_det.get("bounding_box_xyxy", None)
        confidence = obj_det.get("confidence", None)
```

# Visual Anomaly Detection Brick

This Brick lets you perform anomaly detection using a pre-trained neural network model, making it ideal for quality control, monitoring, and automation tasks with Arduino projects.

## Overview

The Visual Anomaly Detection Brick allows you to:

- Process images and identify unusual or abnormal regions.  
- Use local image files, raw byte data, or PIL images as input.  
- Integrate anomaly detection into your Python projects with simple APIs. 

## Features

- Provides **maximum** and **mean anomaly scores** for each processed image.  
- Returns a list of anomaly detections with:  
  - Class label  
  - Anomaly score  
  - Bounding box coordinates (`[x_min, y_min, x_max, y_max]`).  
- Supports multiple input formats: file path, image bytes, or PIL image objects.  
- Simple API with methods for file-based and in-memory detection.  

## Code example and usage

```python
from arduino.app_utils import *
from arduino.app_bricks.visual_anomaly_detection import VisualAnomalyDetection

# Initialize the anomaly detection brick
visual_anomaly = VisualAnomalyDetection()

# Detect from an image file
out = visual_anomaly.detect_from_file("assets/no-anomaly.jpg")

# Detect from image bytes (e.g., read from disk or captured from a camera)
with open("path/to/your/image.png", "rb") as f:
    image_bytes = f.read()

out = visual_anomaly.detect(image_bytes, image_type="png")

# Process the results
if out and "detection" in out:
    print("Anomaly Max Score:", out.get("anomaly_max_score"))
    print("Anomaly Mean Score:", out.get("anomaly_mean_score"))

    for i, anomaly in enumerate(out["detection"]):
        label = anomaly.get("class_name")
        score = anomaly.get("score")
        bbox = anomaly.get("bounding_box_xyxy")
        print(f"{i+1}. {label} - Score: {score}, Box: {bbox}")
```

## Visual Anomaly Detection Working Principle

Visual anomaly detection models are trained to recognize normal patterns in images. When a new image is provided, the model evaluates how much each region deviates from this learned baseline.

The Brick computes two global metrics:

- Maximum anomaly score – the most anomalous region in the image.
- Mean anomaly score – the average anomaly level across all analyzed regions.

Additionally, the model divides the image into a grid and assigns an anomaly score to each section. The Brick extracts these results, associates them with bounding boxes, and returns them in a structured format. Allowing you to not only detect that an image contains anomalies, but also localize where they occur.
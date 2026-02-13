# Vibration Anomaly Detection Brick

This Brick lets you detect vibration anomalies from accelerometer data using a pre-trained Edge Impulse model. It’s ideal for condition monitoring, predictive maintenance, and automation projects.

## Overview

The Vibration Anomaly Detection Brick allows you to:

- Stream accelerometer samples and evaluate the anomaly score per window.
- Trigger a callback automatically when the anomaly score crosses a threshold.
- Integrate quickly via a simple Python API and Arduino Router Bridge.

## Features

- **Edge-Impulse powered**: runs your deployed model via `EdgeImpulseRunnerFacade`.
- **Sliding window ingestion**: samples are buffered to the model’s exact input length.
- **Threshold callbacks**: invoke your handler when `anomaly_score ≥ threshold`.
- **Flexible callback signatures**:
  - `callback()`
  - `callback(anomaly_score: float)`
  - `callback(anomaly_score: float, classification: dict)` (if your model returns a classification head alongside anomaly)

## Code Example and Usage

In the Python® part, use the following script that exposes the `record_sensor_movement` function and analyzes incoming accelerometer data:

```python
from arduino.app_bricks.vibration_anomaly_detection import VibrationAnomalyDetection
from arduino.app_utils import *
import time

logger = Logger("Vibration Anomaly Example")

# Create the Brick with a chosen anomaly threshold
vibration = VibrationAnomalyDetection(anomaly_detection_threshold=1.0)

# Register the callback to run when an anomaly is detected
def on_detected_anomaly(anomaly_score: float, classification: dict = None):
    print(f"[Anomaly] score={anomaly_score:.3f}")

# Expose a function that Arduino can call via Router Bridge
# Expecting accelerations in 'g' from the microcontroller
def record_sensor_movement(x_g: float, y_g: float, z_g: float):
    # Convert to m/s^2 if your model was trained in SI units
    G_TO_MS2 = 9.80665
    x = x_g * G_TO_MS2
    y = y_g * G_TO_MS2
    z = z_g * G_TO_MS2
    # Push a triple (x, y, z) into the sliding window
    vibration.accumulate_samples((x, y, z))

vibration.on_anomaly(on_detected_anomaly)

Bridge.provide("record_sensor_movement", record_sensor_movement)

model_info = vibration.get_model_info()
period = 1.0 / model_info.frequency if model_info and model_info.frequency > 0 else 0.02

# Run the host app (handles Router Bridge and our processing loop)
logger.info(f"Starting App... model_freq={getattr(model_info, 'frequency', 'unknown')}Hz period={period:.4f}s")

App.run()
```

Any accelerometer can provide samples. Here is an example using **Modulino Movement** via Arduino Router Bridge:

```c++
#include <Arduino_RouterBridge.h>
#include <Modulino.h>

// Create a ModulinoMovement object
ModulinoMovement movement;

float x_accel, y_accel, z_accel; // Accelerometer values in g

unsigned long previousMillis = 0; // Stores last time values were updated
const long interval = 10;         // Interval at which to read (10ms) - 100Hz sampling rate, adjust based on model requirements
int has_movement = 0;             // Flag to indicate if movement data is available

void setup() {
  Bridge.begin();

  // Initialize Modulino I2C communication
  Modulino.begin(Wire1);

  // Detect and connect to movement sensor module
  while (!movement.begin()) {
    delay(1000);
  }
}

void loop() {
  unsigned long currentMillis = millis(); // Get the current time

  if (currentMillis - previousMillis >= interval) {
    // Save the last time you updated the values
    previousMillis = currentMillis;

    // Read new movement data from the sensor
    has_movement = movement.update();
    if(has_movement == 1) {
      // Get acceleration values
      x_accel = movement.getX();
      y_accel = movement.getY();
      z_accel = movement.getZ();
    
      Bridge.notify("record_sensor_movement", x_accel, y_accel, z_accel);      
    }
    
  }
}
```

## Working Principle

Vibration anomaly models learn normal accelerometer patterns over time. Each new time-window is compared to that baseline and assigned an anomaly score—higher scores mean the vibration deviates unusually (e.g., new frequencies or amplitudes).

- **Buffering:** Incoming samples are appended to a `SlidingWindowBuffer` sized to your model’s `input_features_count`.
- **Inference:** When a full window is available, features are passed to `EdgeImpulseRunnerFacade.infer_from_features(...)`.
- **Scoring:** The Brick extracts the anomaly score (and any optional classification output) from the inference result.
- **Callback:** If `anomaly_score ≥ anomaly_detection_threshold`, your registered `on_anomaly(...)` callback is invoked.
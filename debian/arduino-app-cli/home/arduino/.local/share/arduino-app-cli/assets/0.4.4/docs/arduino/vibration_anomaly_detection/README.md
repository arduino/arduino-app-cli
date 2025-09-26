# Vibration Anomaly Detection Brick

Leveraging pre-trained AI models, this brick enables vibration anomaly detection by processing accelerometer samples to identify anomalies in vibration patterns.
It can integrate with models provided by the framework or custom anomaly detection models trained via the Edge Impulse platform.

## Code example and usage

```python
from arduino.app_bricks.vibration_anomaly_detection import VibrationAnomalyDetection
from arduino.app_utils import App

# For more information about anomaly score, please refers to: https://docs.edgeimpulse.com/studio/projects/learning-blocks/blocks/anomaly-detection-gmm
vibration_detection = VibrationAnomalyDetection(anomaly_detection_threshold=1.0)

# Register function to receive samples from sketch
def record_sensor_movement(x: float, y: float, z: float):
  # Acceleration from sensor is in g. While we need m/s^2.
  x = x * 9.81
  y = y * 9.81
  z = z * 9.81
  
  # Append the values to the sensor buffer. These samples will be sent to the model.
  global motion_devibration_detectiontection
  vibration_detection.accumulate_samples((x, y, z))

Bridge.provide("record_sensor_movement", record_sensor_movement)

# Register action to take after successful detection
def on_detected_anomaly(anomaly_score: float, classification: dict):
    print(f"Anomaly detected. Score: {anomaly_score}")

vibration_detection.on_anomaly(on_detected_anomaly)

App.run()
```

samples can be provided by accelerometer connected to microcontroller.
Here is an examples using a Modulino Movement accelerometer.

```c++
#include <Arduino_RouterBridge.h>
#include <Modulino.h>

// Create a ModulinoMovement object
ModulinoMovement movement;

float x_accel, y_accel, z_accel; // Accelerometer values in g

unsigned long previousMillis = 0; // Stores last time values were updated
const long interval = 16;         // Interval at which to read (16ms) - sampling rate of 62.5Hz and should be adjusted based on model definition
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

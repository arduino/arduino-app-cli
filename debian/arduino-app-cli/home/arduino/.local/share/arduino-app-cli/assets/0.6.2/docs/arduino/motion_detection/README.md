# Motion Detection Brick

Leveraging pre-trained AI models, this brick enables motion detection by processing accelerometer samples to identify specific movements.

It can integrate with models provided by the framework or custom models trained via the Edge Impulse platform.

## Overview

The Motion Detection brick allows you to:

- Process accelerometer data to identify specific motion patterns
- Use pre-trained models provided by the framework
- Integrate custom motion classification models trained on the Edge Impulse platform
- Register callbacks for detected motion patterns
- Configure detection confidence levels

It analyzes accelerometer samples in real-time to classify motion patterns such as gestures, activities or specific movements. The brick manages data buffering, model inference and provides callback mechanisms for handling detected motions.

## Features

- Real-time motion pattern classification using accelerometer data
- Configurable confidence thresholds for detection accuracy
- Automatic data buffering with sliding window processing
- Callback-based event handling for detected motion patterns
- Support for custom Edge Impulse trained models
- Thread-safe motion detection processing

## Code example and usage

```python
from arduino.app_bricks.motion_detection import MotionDetection
from arduino.app_utils import App

motion_detection = MotionDetection(confidence=0.4)

# Register function to receive samples from sketch
def record_sensor_movement(x: float, y: float, z: float):
  # Acceleration from sensor is in g. While we need m/s^2.
  x = x * 9.81
  y = y * 9.81
  z = z * 9.81
  
  # Append the values to the sensor buffer. These samples will be sent to the model.
  global motion_detection
  motion_detection.accumulate_samples((x, y, z))

Bridge.provide("record_sensor_movement", record_sensor_movement)

# Register action to take after successful detection
def on_updown_movement_detected(classification: dict):
    print(f"updown movement detected!")

motion_detection.on_movement_detection('updown', on_updown_movement_detected)

App.run()
```

Samples can be provided by accelerometer connected to microcontroller.

Here is an example using a Modulino Movement accelerometer.

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

## Understanding Motion Detection

The Motion Detection brick processes accelerometer data through a sliding window buffer system, analyzing patterns of movement to classify specific motions or gestures. The confidence parameter determines the minimum threshold required for a motion pattern to be considered detected.

Motion patterns are identified using machine learning models that have been trained to recognize specific movement signatures. Each detected motion includes a confidence score indicating the model's certainty about the classification, allowing you to filter results based on your reliability requirements.
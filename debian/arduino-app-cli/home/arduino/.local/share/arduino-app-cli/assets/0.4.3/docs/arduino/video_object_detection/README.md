# Video Object Detection Brick

This object detection brick utilizes a pre-trained model to analyze video streams from a camera.
It identifies objects, returning their predicted class labels, bounding boxes, and confidence scores.
The output is a video stream featuring bounding boxes around detected objects, with the added capability to trigger actions based on these detections.
It supports pre-trained models provided by the framework and custom object detection models trained on the Edge Impulse platform.

## Code example and usage

```python
from arduino.app_utils import App
from arduino.app_bricks.video_objectdetection import VideoObjectDetection

detection_stream = VideoObjectDetection()

def person_detected():
  print("Detected a person!!!")

detection_stream.on_detect("person", person_detected)

App.run()
```

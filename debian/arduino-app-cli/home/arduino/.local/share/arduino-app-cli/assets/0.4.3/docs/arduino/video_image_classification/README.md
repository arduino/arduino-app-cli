# Video Image Classification Brick

This image classification brick utilizes a pre-trained model to analyze video streams from a camera.
It identifies objects, returning their predicted class labels and confidence scores.
The output is a video stream featuring classification as overaly, with the added capability to trigger actions based on these detections.
It supports pre-trained models provided by the framework and custom object detection models trained on the Edge Impulse platform.

## Code example and usage

```python
from arduino.app_utils import App
from arduino.app_bricks.video_imageclassification import VideoImageClassification

detection_stream = VideoImageClassification()

def dog_detected():
  print("Detected a dog!!!")

detection_stream.on_detect("dog", dog_detected)

App.run()
```

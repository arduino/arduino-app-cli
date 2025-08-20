# Audio Classifier

Brick for classifying audio input using a pre-trained model that processes a continuous audio stream to detect specific sounds.

## Features
- Supports audio classification using a pre-trained model.
- Processes audio in real-time.
- Classifies sounds from existing audio files.

## Code example and usage

```python
from arduino.app_bricks.audio_classifier import AudioClassifier
from arduino.app_utils import App

classifier = AudioClassifier()
classifier.on_detect("Glass_Breaking", lambda: print(f"Glass breaking sound detected!"))

App.run()
```

or using an existing audio file:

```python
from arduino.app_bricks.audio_classifier import AudioClassifier

classifier = AudioClassifier()
classification = classifier.classify_from_file("glass_breaking.wav")
print("Result:", classification)
```

# audio_classifier API Reference

## Index

- Class `AudioClassifierException`
- Class `AudioClassifier`

---

## `AudioClassifierException` class

```python
class AudioClassifierException()
```

Custom exception for AudioClassifier errors.


---

## `AudioClassifier` class

```python
class AudioClassifier(mic: Microphone, confidence: float)
```

AudioClassifier module for detecting sounds and classifying audio using a specified model.

### Parameters

- **mic** (*Microphone*): Microphone instance for audio input. If None, a default Microphone will be initialized.
- **confidence** (*float*): Confidence level for detection. Default is 0.8 (80%).

### Raises

- **ValueError**: If the model information cannot be retrieved or if the model parameters are incomplete.

### Methods

#### `AudioClassifier.on_detect(class_name: str, callback: Callable[[], None])`

Register a callback function to be invoked when a specific class is detected.

##### Parameters

- **class_name** (*str*): The class to check for in the classification results.
- **callback** (*callable*): a callback function to handle the keyword spotted.

#### `AudioClassifier.start()`

Start the AudioClassifier module and begin processing audio data.

#### `AudioClassifier.stop()`

Stop the AudioClassifier module and release resources.

#### `AudioClassifier.classify_from_file(audio_path: str, confidence: int)`

Classify audio from a file.

##### Parameters

- **audio_path** (*str*): Path to the audio file.
- **confidence** (*int*): Confidence threshold for classification. If None, the default confidence level is used.

##### Returns

-: dict | None: Classification results or None if the path is invalid.

##### Raises

- **AudioClassifierException**: If the audio file cannot be read or processed.


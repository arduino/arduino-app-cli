# asr API Reference

## Index

- Class `AutomaticSpeechRecognition`
- Class `ASREvent`
- Class `TranscriptionStream`

---

## `AutomaticSpeechRecognition` class

```python
class AutomaticSpeechRecognition(language: str | None)
```

### Parameters

- **language**: The language code for the ASR model (e.g., "en" for English).

### Methods

#### `start()`

Prepare the ASR for transcription.

#### `stop()`

Stop the ASR and clean up resources.

#### `cancel()`

Cancel all active transcription sessions.

#### `transcribe_mic(mic: BaseMicrophone, duration: int)`

Transcribe audio data from the microphone and return the transcribed text.

#### `transcribe_mic_stream(mic: BaseMicrophone, duration: int)`

Transcribe audio data from the microphone and stream the results as soon as they are available.

#### `transcribe_wav(wav_data: np.ndarray | bytes)`

Transcribe audio from WAV data and return the transcribed text.

#### `transcribe_wav_stream(wav_data: np.ndarray | bytes)`

Transcribe audio from WAV data and stream the results.


---

## `ASREvent` class

```python
class ASREvent()
```


---

## `TranscriptionStream` class

```python
class TranscriptionStream(generator: Generator[T, None, None])
```

Iterator wrapper that guarantees proper teardown on context exit.


# tts API Reference

## Index

- Class `TextToSpeech`
- Class `TTSError`
- Class `TTSBusyError`
- Class `SynthesisStream`

---

## `TextToSpeech` class

```python
class TextToSpeech(speaker: BaseSpeaker | None)
```

Text-to-Speech brick for offline speech synthesis using local TTS service.

### Parameters

- **speaker** (*BaseSpeaker*) (optional): Speaker instance to use for audio output. If not provided, a default Speaker will be used.

### Methods

#### `start()`

Start the TextToSpeech brick by initializing the speaker.

#### `stop()`

Stop the TextToSpeech brick by stopping the speaker.

#### `cancel()`

Cancel active speech playback, if any, without stopping the speaker.

#### `speak(text: str)`

Synthesize speech from text and play it through the provided speaker.

Long text is split into 1024-character chunks before synthesis.

##### Parameters

- **text** (*str*): The text to be synthesized into speech.

##### Raises

- **TTSBusyError**: If this instance already has an active speech session.
- **RuntimeError**: If the synthesis fails.

#### `synthesize_wav(text: str)`

Synthesize speech from text and return the audio in WAV format.

##### Parameters

- **text** (*str*): The text to be synthesized into speech.

##### Returns

- (*bytes*): The synthesized audio in WAV format.

##### Raises

- **TTSBusyError**: If this instance already has an active speech session.
- **RuntimeError**: If the synthesis fails.

#### `synthesize_pcm(text: str)`

Synthesize speech from text and return the audio in PCM format (mono, 16-bit, 44.1kHz).

##### Parameters

- **text** (*str*): The text to be synthesized into speech.

##### Returns

- (*bytes*): The synthesized audio in PCM format.

##### Raises

- **TTSBusyError**: If this instance already has an active speech session.
- **RuntimeError**: If the synthesis fails.

#### `synthesize_pcm_stream(text: str)`

Synthesize speech from text and stream PCM audio chunks as they arrive.

##### Parameters

- **text** (*str*): The text to be synthesized into speech.

##### Returns

- (*SynthesisStream*): An iterable/context-manager yielding PCM audio chunks. Use as a
``with`` block to guarantee teardown of the underlying HTTP response and
release of the session lock.

##### Raises

- **TTSBusyError**: If this instance already has an active speech session.
- **RuntimeError**: If the synthesis fails.


---

## `TTSError` class

```python
class TTSError()
```

Base class for TTS errors.


---

## `TTSBusyError` class

```python
class TTSBusyError()
```

Raised when this TTS instance already has an active speech session.


---

## `SynthesisStream` class

```python
class SynthesisStream(generator: Generator[bytes, None, None])
```

Iterator wrapper that guarantees proper teardown on context exit.


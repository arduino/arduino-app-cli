# asr API Reference

## Index

- Class `ASREvent`
- Class `ASRError`
- Class `ASRBusyError`
- Class `ASRServiceBusyError`
- Class `ASRUnavailableError`
- Class `TranscriptionStream`
- Class `AutomaticSpeechRecognition`
- Class `WAVAutomaticSpeechRecognition`

---

## `ASREvent` class

```python
class ASREvent()
```


---

## `ASRError` class

```python
class ASRError()
```

Base class for ASR errors.


---

## `ASRBusyError` class

```python
class ASRBusyError()
```

Raised when this ASR instance already has an active transcription session.


---

## `ASRServiceBusyError` class

```python
class ASRServiceBusyError()
```

Raised when the inference server rejects session creation because it is serving another client.


---

## `ASRUnavailableError` class

```python
class ASRUnavailableError()
```

Raised when the inference service is unreachable or the connection drops unexpectedly.


---

## `TranscriptionStream` class

```python
class TranscriptionStream(generator: Generator[T, None, None])
```

Iterator wrapper that guarantees proper teardown on context exit.


---

## `AutomaticSpeechRecognition` class

```python
class AutomaticSpeechRecognition(mic: BaseMicrophone | None, language: str | None)
```

ASR brick for live audio transcription from a microphone.

### Parameters

- **mic**: Microphone to be captured for transcription. One of:
BaseMicrophone: used as-is; the caller owns its
    lifecycle (ASR never calls start()/stop() on it).
None: ASR constructs a default Microphone() and owns its
    lifecycle (started on start(), stopped on stop()).
Default: None.
- **language** (*str*): Language code for the ASR model (e.g. "en" for
English). This is typically auto-detected by the model,
but can be overridden here if needed. It is exposed as
the public ``language`` attribute and may be reassigned at
runtime; the new value takes effect on the next session.

### Methods

#### `transcribe(duration: int)`

Transcribe audio for a duration and return the final text.

##### Parameters

- **duration** (*int*): Maximum recording time in seconds. ``0`` means unbounded.
Default: ``60``.

##### Returns

- (*str*): The transcribed text, or an empty string if no speech was detected.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the
connection drops mid-session.
- **RuntimeError**: If the microphone has not been started.

#### `transcribe_stream(duration: int)`

Transcribe audio for a duration and yield intermediate transcription events.

##### Parameters

- **duration** (*int*): Maximum recording time in seconds. ``0`` means unbounded.
Default: ``0``.

##### Returns

- (*ASREvent*): objects representing transcription events.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the
connection drops mid-session.
- **RuntimeError**: If the microphone has not been started.

#### `transcribe_sentence(timeout: int)`

Transcribe a sentence returning the full text.

Runs until the sentence boundary is detected, the timeout elapses
without one.

##### Parameters

- **timeout** (*int*): Maximum recording time in seconds. ``0`` means no timeout.
Default: ``0``.

##### Returns

- (*str*): The transcribed text, or an empty string if no speech was detected.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the connection drops mid-session.
- **RuntimeError**: If the microphone has not been started.

#### `transcribe_sentence_stream(timeout: int)`

Transcribe a sentence and yield the intermediate transcription events.

The stream ends after the sentence boundary is detected, the timeout
elapses without one.

##### Parameters

- **timeout** (*int*): Maximum recording time in seconds. ``0`` means no timeout.
Default: ``0``.

##### Returns

- (*ASREvent*): objects representing transcription events.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the
connection drops mid-session.
- **RuntimeError**: If the microphone has not been started.

#### `transcribe_until_cancelled()`

Transcribe audio indefinitely and yield intermediate transcription events.

The stream ends only when :meth:`cancel` is called.

##### Returns

- (*ASREvent*): objects representing transcription events.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the
connection drops mid-session.
- **RuntimeError**: If the microphone has not been started.


---

## `WAVAutomaticSpeechRecognition` class

```python
class WAVAutomaticSpeechRecognition(wav: np.ndarray | bytes, language: str | None)
```

ASR brick for offline transcription of in-memory audio.

### Parameters

- **wav**: WAV data to be used for transcription. One of:
np.ndarray: treated as raw PCM samples at 16 kHz mono.
bytes: treated as a WAV container.
- **language** (*str*): Language code for the ASR model (e.g. "en" for
English). This is typically auto-detected by the model,
but can be overridden here if needed. It is exposed as
the public ``language`` attribute and may be reassigned at
runtime; the new value takes effect on the next session.

### Methods

#### `transcribe()`

Consume the WAV to completion and return the transcribed text.

##### Returns

- (*str*): The transcribed text, or an empty string if no speech was detected.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the
connection drops mid-session.

#### `transcribe_stream()`

Consume the WAV to completion and yield transcription events.

##### Returns

- (*ASREvent*): objects representing transcription events.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the
connection drops mid-session.


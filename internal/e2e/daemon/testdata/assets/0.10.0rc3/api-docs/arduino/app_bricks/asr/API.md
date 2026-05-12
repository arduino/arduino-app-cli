# asr API Reference

## Index

- Class `ASREvent`
- Class `ASRError`
- Class `ASRBusyError`
- Class `ASRServiceBusyError`
- Class `ASRUnavailableError`
- Class `AutomaticSpeechRecognition`
- Class `TranscriptionStream`

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

## `AutomaticSpeechRecognition` class

```python
class AutomaticSpeechRecognition(source: BaseMicrophone | np.ndarray | bytes | None, language: str | None)
```

### Parameters

- **source**: Audio source for transcription. One of:
BaseMicrophone: used as-is; the caller owns its
    lifecycle (ASR never calls start()/stop() on it).
bytes: treated as a WAV container and wrapped internally.
np.ndarray: treated as raw PCM samples at 16 kHz mono
    (dtype inferred) and wrapped internally.
None: ASR constructs a default Microphone() and owns its
    lifecycle (started on start(), stopped on stop()).
Default: None.
- **language** (*str*): Language code for the ASR model (e.g. "en" for
English). This is typically auto-detected by the model,
but can be overridden here if needed.

### Methods

#### `start()`

Prepare the ASR for transcription. Starts the owned mic if applicable.

#### `stop()`

Stop the ASR and clean up resources. Stops the owned mic if applicable.

#### `cancel()`

Cancel the active transcription session, if any.

#### `transcribe(duration: int)`

Transcribe audio from the configured source and return the final text.

##### Parameters

- **duration** (*int*): Maximum recording time in seconds. ``0`` means unbounded.
Ignored for finite sources (WAV/ndarray), which are consumed
to completion regardless. Default: ``0``.

##### Returns

- (*str*): The transcribed text, or an empty string if no speech was detected.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the
connection drops mid-session.
- **RuntimeError**: If the audio source has not been started.

#### `transcribe_stream(duration: int)`

Transcribe audio from the configured source and stream events.

##### Parameters

- **duration** (*int*): Maximum recording time in seconds. ``0`` means unbounded.
Ignored for finite sources (WAV/ndarray). Default: ``0``.

##### Returns

- (*ASREvent*): objects representing transcription events.

##### Raises

- **ASRBusyError**: If this instance already has an active session.
- **ASRServiceBusyError**: If no more concurrent sessions are available.
- **ASRUnavailableError**: If the inference service is unreachable or the
connection drops mid-session.
- **RuntimeError**: If the audio source has not been started.


---

## `TranscriptionStream` class

```python
class TranscriptionStream(generator: Generator[T, None, None])
```

Iterator wrapper that guarantees proper teardown on context exit.


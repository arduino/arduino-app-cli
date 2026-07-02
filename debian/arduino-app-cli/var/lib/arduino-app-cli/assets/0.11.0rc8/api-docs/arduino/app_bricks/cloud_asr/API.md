# cloud_asr API Reference

## Index

- Class `CloudASR`

---

## `CloudASR` class

```python
class CloudASR(api_key: str, provider: CloudProvider, mic: BaseMicrophone | None, language: str, silence_timeout: float)
```

Cloud-based speech-to-text with pluggable cloud providers.

It captures audio from a microphone and streams it to the selected cloud ASR provider for transcription.
The recognized text is yielded as events in real-time.

### Methods

#### `start()`

Start the ASR service by initializing the microphone.

#### `stop()`

Stop the ASR service: signal in-flight transcriptions and release

the mic if owned.

#### `cancel()`

Cancel the active transcription session, if any.

#### `is_transcribing()`

Return True if a transcription session is currently active on this instance.

#### `transcribe(duration: float)`

Returns the first utterance transcribed from speech to text.

##### Parameters

- **duration** (*float*): Max seconds for the transcription session.
``0`` means unbounded.

##### Returns

- (*str*): The transcribed text.

#### `transcribe_stream(duration: float)`

Perform continuous speech-to-text recognition.

##### Parameters

- **duration** (*float*): Max seconds for the transcription session.
``0`` means unbounded.

##### Returns

- (*Iterator[ASREvent]*): Generator yielding transcription events.

#### `transcribe_sentence(timeout: float)`

Transcribe a single sentence and return its text.

Stops at the first sentence boundary produced by the provider, or when
``timeout`` elapses. VAD is managed by the cloud provider.

##### Parameters

- **timeout** (*float*): Max seconds to wait for the sentence.
``0`` means no timeout.

##### Returns

- (*str*): The transcribed sentence.

#### `transcribe_sentence_stream(timeout: float)`

Yield transcription events for a single sentence.

The stream ends after the first ``text`` event or when ``timeout``
elapses. VAD is managed by the cloud provider.

##### Parameters

- **timeout** (*float*): Max seconds to wait for the sentence.
``0`` means no timeout.

##### Returns

- (*ASREvent*): Transcription events.

#### `transcribe_until_cancelled()`

Yield one sentence per ``text`` event until ``cancel()`` is called

or the silence timeout fires. VAD is managed by the cloud provider.

##### Returns

- (*str*): A complete sentence as recognized by the provider.


# cloud_asr API Reference

## Index

- Class `CloudASR`

---

## `CloudASR` class

```python
class CloudASR(api_key: str, provider: CloudProvider, mic: Optional[Microphone], language: str, silence_timeout: float)
```

Cloud-based speech-to-text with pluggable cloud providers.

It captures audio from a microphone and streams it to the selected cloud ASR provider for transcription.
The recognized text is yielded as events in real-time.

### Methods

#### `transcribe(duration: float)`

Returns the first utterance transcribed from speech to text.

##### Parameters

- **duration** (*float*): Max seconds for the transcription session.

##### Returns

- (*str*): The transcribed text.

#### `transcribe_stream(duration: float)`

Perform continuous speech-to-text recognition.

##### Parameters

- **duration** (*float*): Max seconds for the transcription session.

##### Returns

- (*Iterator[ASREvent]*): Generator yielding transcription events.


# tts API Reference

## Index

- Class `TextToSpeech`

---

## `TextToSpeech` class

```python
class TextToSpeech(language: str | None, speaker: BaseSpeaker | None)
```

Text-to-Speech brick for offline speech synthesis using local TTS service.

### Parameters

- **language** (*str*) (optional): Preferred language for TTS. If not specified, it follow App configuration.
- **speaker** (*BaseSpeaker*) (optional): Speaker instance to use for audio output. If not provided, a default Speaker will be used.

### Methods

#### `start()`

Start the TextToSpeech brick by initializing the speaker.

#### `stop()`

Stop the TextToSpeech brick by stopping the speaker.

#### `speak(text: str)`

Synthesize speech from text and play it through the provided speaker.

##### Parameters

- **text** (*str*): The text to be synthesized into speech.

##### Raises

- **ValueError**: If the specified language is not supported.
- **RuntimeError**: If the synthesis fails or maximum concurrency is reached.

#### `synthesize_wav(text: str)`

Synthesize speech from text and return the audio in WAV format.

##### Parameters

- **text** (*str*): The text to be synthesized into speech.

##### Returns

- (*bytes*): The synthesized audio in WAV format.

##### Raises

- **ValueError**: If the specified language is not supported.
- **RuntimeError**: If the synthesis fails or maximum concurrency is reached.

#### `synthesize_pcm(text: str, language: Literal['en', 'es', 'zh'])`

Synthesize speech from text and return the audio in PCM format (mono, 16-bit, 44.1kHz).

##### Parameters

- **text** (*str*): The text to be synthesized into speech.
- **language** (*Literal["en", "es", "zh"]*): The language of the text.

##### Returns

- (*bytes*): The synthesized audio in PCM format.

##### Raises

- **ValueError**: If the specified language is not supported.
- **RuntimeError**: If the synthesis fails or maximum concurrency is reached.


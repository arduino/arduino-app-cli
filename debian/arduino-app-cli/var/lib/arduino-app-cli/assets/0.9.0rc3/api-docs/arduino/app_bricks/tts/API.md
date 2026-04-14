# tts API Reference

## Index

- Class `TextToSpeech`

---

## `TextToSpeech` class

```python
class TextToSpeech()
```

Text-to-Speech brick for offline speech synthesis using local TTS service.

### Methods

#### `speak(text: str, language: Literal['en', 'es', 'zh'], speaker: BaseSpeaker | None)`

Synthesize speech from text and play it through the provided speaker.

##### Parameters

- **text** (*str*): The text to be synthesized into speech.
- **language** (*Literal["en", "es", "zh"]*): The language of the text.
- **speaker** (*BaseSpeaker*): The speaker instance to play the synthesized audio.
If None, a default Speaker will be used.

##### Raises

- **ValueError**: If the specified language is not supported.
- **RuntimeError**: If the synthesis fails or maximum concurrency is reached.

#### `synthesize_wav(text: str, language: Literal['en', 'es', 'zh'])`

Synthesize speech from text and return the audio in WAV format.

##### Parameters

- **text** (*str*): The text to be synthesized into speech.
- **language** (*Literal["en", "es", "zh"]*): The language of the text.

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


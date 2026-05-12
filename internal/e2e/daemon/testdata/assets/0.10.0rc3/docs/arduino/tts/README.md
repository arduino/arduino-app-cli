# Text To Speech (TTS) Brick

The `TextToSpeech` brick provides a completely offline text-to-speech (TTS) solution for Arduino Apps. It's designed to convert text input into spoken audio using locally available TTS engines, ensuring privacy and low-latency performance without reliance on cloud services.

## Key Features

- **Offline Operation:** All speech synthesis is performed locally, ensuring data privacy and eliminating network dependencies.
- **Multiple Language Support:** Select a language by configuring the corresponding TTS model (e.g. `melo-tts-en`, `melo-tts-es`, `melo-tts-zh`, `piper-tts-de`, `piper-tts-en`, `piper-tts-it`) in `brick_config.yaml` or override per-app in `app.yaml`.
- **Audio Output Formats:** Directly output synthesized speech to a Speaker instance or to WAV, PCM, or PCM audio.
- **Long Text Support:** `speak()` splits long input into sentence-aware chunks of up to 1024 characters before synthesis.
- **Streaming Playback:** `speak()` plays PCM chunks as they arrive from the local TTS service instead of waiting for the full rendered response.
- **Cancellable Playback:** Use `cancel()` to stop the current spoken sequence and notify the local TTS service without stopping the TTS brick or speaker.
- **Single-Session Semantics:** Each instance handles one speech session at a time. For concurrent speech, create multiple `TextToSpeech` instances.

## Errors

- `TTSBusyError`: raised if you call `speak()`, `synthesize_pcm()`, `synthesize_pcm_stream()`, or `synthesize_wav()` while the instance already has an active session. Fix by awaiting the current session or using a separate instance.
- `TTSError`: base class for all of the above.

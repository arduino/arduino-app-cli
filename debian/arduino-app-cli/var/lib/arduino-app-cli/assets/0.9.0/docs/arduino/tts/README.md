# Text To Speech (TTS) Brick

The `TextToSpeech` brick provides a completely offline text-to-speech (TTS) solution for Arduino Apps. It's designed to convert text input into spoken audio using locally available TTS engines, ensuring privacy and low-latency performance without reliance on cloud services.

## Key Features

- **Offline Operation:** All speech synthesis is performed locally, ensuring data privacy and eliminating network dependencies.
- **Multiple Language Support:** Easily switch between different languages (en, es, zh).
- **Audio Output Formats:** Directly output synthesized speech to a Speaker instance or to WAV or PCM audio.
- **Concurrency Control**: Limits the number of simultaneous transcription sessions to avoid resource exhaustion.

# Automatic Speech Recognition Brick

The `AutomaticSpeechRecognition` brick provides on-device automatic speech recognition (ASR) capabilities for audio streams and files. It offers a high-level interface for transcribing audio using a local model, with support for both real-time microphone capture and in-memory audio (WAV bytes or raw PCM arrays).

## Features

- **Offline Operation:** All transcriptions are performed locally, ensuring data privacy and eliminating network dependencies.
- **Multi-Language Support:** Supports the transcription of multiple spoken languages.
- **Flexible Audio Input:** The constructor accepts a `BaseMicrophone` instance, a `bytes` WAV container, a raw `np.ndarray` of PCM samples, or `None` to use a default `Microphone()`.
- **Single-Session Semantics:** Each instance handles one transcription session at a time. For concurrent transcriptions on different microphones, create multiple `AutomaticSpeechRecognition` instances.

## Errors

- `ASRBusyError`: raised if you call `transcribe()` / `transcribe_stream()` while the instance already has an active session. Fix by awaiting the current session or using a separate instance.
- `ASRServiceBusyError`: raised when the inference server rejects session creation because it is currently serving another client. The caller decides whether to retry.
- `ASRUnavailableError`: raised when the inference service is unreachable (container down, network error) or the WebSocket connection drops mid-session. The caller decides whether to retry.
- `ASRError`: base class for all of the above.

## Source Ownership

- When `source` is `None`, ASR constructs a default `Microphone()` and manages its lifecycle through `asr.start()` / `asr.stop()`.
- When `source` is a `BaseMicrophone` you pass in, **you** own its lifecycle — call `mic.start()` before transcribing and `mic.stop()` when done.
- In-memory sources (`bytes`, `np.ndarray`) have no device lifecycle.

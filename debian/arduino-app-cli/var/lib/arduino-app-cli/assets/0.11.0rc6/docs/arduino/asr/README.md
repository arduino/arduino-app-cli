# Automatic Speech Recognition Brick

The `AutomaticSpeechRecognition` brick provides on-device automatic speech recognition (ASR) capabilities for audio streams and files. It offers a high-level interface for transcribing audio using a local model, with support for both real-time microphone capture and in-memory audio (WAV bytes or raw PCM arrays). With the possibility to use multiple languages.

## Features

- **Offline Operation:** All transcriptions are performed locally, ensuring data privacy and eliminating network dependencies.
- **Multi-Language Support:** Supports the transcription of multiple spoken languages.
- **Flexible Audio Input:** The constructor accepts a `BaseMicrophone` instance, a `bytes` WAV container, a raw `np.ndarray` of PCM samples, or `None` to use a default `Microphone()`.
- **Single-Session Semantics:** Each instance handles one transcription session at a time. For concurrent transcriptions on different microphones, create multiple `AutomaticSpeechRecognition` instances.

## Prerequisites

Before using the ASR brick, ensure you have the following components:

- USB microphone
OR
- WAV or PCM audio file

Tips:
- Use a USB-C® Hub with USB-A connectors to support commercial USB cameras with microphone. Note that the USB-C® Hub must have Power Delivery Support (PD).
- Microphones included in USB cameras/webcams are generally supported

## LocalASR Class Features

- All transcriptions are performed locally, ensuring data privacy and eliminating network dependencies.
- Supports the transcription of multiple spoken languages.
- Works with the Microphone peripheral as well as WAV and PCM audio files.
- Limits the number of simultaneous transcription sessions to avoid resource exhaustion.

## Code Example and Usage

This example transcribes audio captured from the microphone for 5 seconds. The brick automatically uses the microphone and handles the start and stop functions.

```python
from arduino.app_bricks.asr import AutomaticSpeechRecognition
from arduino.app_peripherals.microphone import Microphone

asr = AutomaticSpeechRecognition()
text = asr.transcribe_mic(mic, duration=5)
print(f"Transcription: {text}")

mic.stop()
```

This example transcribes audio from a file.

```python
from arduino.app_bricks.asr import AutomaticSpeechRecognition


asr = AutomaticSpeechRecognition()
with open("recording_01.wav", "rb") as wav_file:
    text = asr.transcribe_wav(wav_file.read())
    print(f"Transcription: {text}")
```

## Errors

- `ASRBusyError`: raised if you call `transcribe()` / `transcribe_stream()` while the instance already has an active session. Fix by awaiting the current session or using a separate instance.
- `ASRServiceBusyError`: raised when the inference server rejects session creation because it is currently serving another client. The caller decides whether to retry.
- `ASRUnavailableError`: raised when the inference service is unreachable (container down, network error) or the WebSocket connection drops mid-session. The caller decides whether to retry.
- `ASRError`: base class for all of the above.

## Source Ownership

- When `source` is `None`, ASR constructs a default `Microphone()` and manages its lifecycle through `asr.start()` / `asr.stop()`.
- When `source` is a `BaseMicrophone` you pass in, **you** own its lifecycle — call `mic.start()` before transcribing and `mic.stop()` when done.
- In-memory sources (`bytes`, `np.ndarray`) have no device lifecycle.

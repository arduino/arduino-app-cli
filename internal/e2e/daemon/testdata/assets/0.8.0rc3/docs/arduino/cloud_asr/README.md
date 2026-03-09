# Cloud ASR Brick

The Cloud ASR Brick provides a unified interface to cloud-based Automatic Speech Recognition (ASR) services (OpenAI and Google) to convert microphone audio into text.

## Overview

This Brick streams audio from a `Microphone` to a selected provider and yields transcription events.

## Features

- **Multi-provider**: `CloudProvider.OPENAI_TRANSCRIBE` or `CloudProvider.GOOGLE_SPEECH`.
- **Streaming events**: `speech_start`, `partial_text`, `speech_stop`, `text`.
- **Configurable**: language,silence timeout and overall timeout.

## Prerequisites

- Microphone + internet connection.
- Provider API key (set via App Lab Brick Configuration as `API_KEY`).
- Optional deps: `arduino_app_bricks[cloud_asr]`.

## Code Example and Usage

This example streams events and stops when the user says "stop".

```python
from arduino.app_bricks.cloud_asr import CloudASR
from arduino.app_utils import App

asr = CloudASR()

def stream_events():
    print("Say 'stop' to stop the transcription.")
    with asr.transcribe_stream(duration=120.0) as events:
        for event in events:
            print(f"{event.type}: {event.data}")
            if event.type == "text" and (event.data or "").strip().lower() == "stop":
                break

App.run(stream_events)
```

## Configuration

The Brick is initialized with the following parameters:

| Parameter         | Type                     | Default                                | Description                                                                                                                        |
| :---------------- | :----------------------- | :------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------------- |
| `api_key`         | `str`                    | `os.getenv("API_KEY", "")`             | API key for the selected provider. **Recommended:** set via the **Brick Configuration** menu in App Lab instead of code.          |
| `provider`        | `CloudProvider`          | `CloudProvider.OPENAI_TRANSCRIBE`      | Cloud provider to use for transcription.                                                                                           |
| `mic`             | `Microphone \| None`     | `None`                                 | Optional microphone instance. If not provided, a `Microphone()` peripheral is created.                                             |
| `language`        | `str`                    | `os.getenv("LANGUAGE", "")`            | Language code for transcription (e.g., `en`, `it`). If empty, the provider falls back to its default (typically English).         |
| `silence_timeout` | `float`                  | `10.0`                                 | Stops the session if no speech (partial or final text) is detected for this many seconds.                                         |

### Supported Providers

You can select a provider using the `CloudProvider` enum or by passing its raw string value.

| Enum Constant                       | Raw String ID         | Provider Documentation                                                                 |
| :---------------------------------- | :-------------------- | :----------------------------------------------------------------------------------- |
| `CloudProvider.OPENAI_TRANSCRIBE`   | `openai-transcribe`   | [GPT-4o-mini-transcribe](https://platform.openai.com/docs/models/gpt-4o-mini-transcribe), [OpenAI Realtime](https://platform.openai.com/docs/guides/realtime)                   |
| `CloudProvider.GOOGLE_SPEECH`       | `google-speech`       | [Google Speech-to-Text](https://cloud.google.com/speech-to-text/docs)                 |

## Methods

- **`transcribe(duration=60.0)`**: Returns the first finalized transcription (`text`).
- **`transcribe_stream(duration=60.0)`**: Yields `ASREvent` items (`speech_start`, `partial_text`, `text`, `speech_stop`).

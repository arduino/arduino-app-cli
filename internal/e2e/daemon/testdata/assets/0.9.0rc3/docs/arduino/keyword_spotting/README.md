# Keyword Spotter Brick

Brick for keyword spotting using a pre-trained model that processes a continuous audio stream to detect specific keywords or phrases.

## Overview

The Keyword Spotter brick allows you to:

- Detect specific keywords in real-time audio streams
- Use pre-trained models provided by the framework  
- Integrate custom audio classification models trained on the Edge Impulse platform
- Configure detection confidence levels and debounce timing
- Register callback functions for keyword detection events

It processes audio input through a microphone to classify and detect targeted keywords or phrases. The brick supports both framework-provided models and custom models trained on the Edge Impulse platform, making it flexible for custom keyword detection applications.

## Prerequisites

Before using the Keyword Spotter brick, ensure you have the following components:

- USB microphone

Tips:
- Use a USB-C® Hub with USB-A connectors to support commercial USB cameras with microphone.
- Microphones included in USB camera/webcams are generally supported

## Features

- Real-time audio processing with continuous stream analysis
- Configurable confidence thresholds for detection accuracy
- Debounce functionality to prevent repeated detections  
- Callback-based event handling for detected keywords
- Support for custom Edge Impulse trained models
- Default microphone initialization when no mic specified

## Code example and usage

Here is a basic example for detecting the 'hello world' keyword:

```python
from arduino.app_bricks.keyword_spotter import KeywordSpotter
from arduino.app_utils import App

spotter = KeywordSpotter()
spotter.on_detect("helloworld", lambda: print(f"Hello world detected!"))

App.run()
```

You can customize the confidence level and debounce timing:

```python
spotter = KeywordSpotter(confidence=0.9, debounce_sec=3.0)
```

## Understanding Detection Parameters

The KeywordSpotter uses three key configuration parameters:

- The `confidence` parameter sets the minimum confidence level required for a detection, with higher values reducing false positives but potentially missing valid detections.
- The `debounce_sec` parameter prevents repeated detection callbacks for the same keyword within the specified time window.
- The `mic` parameter allows you to specify a custom Microphone instance. Otherwise, it defaults to a standard microphone.
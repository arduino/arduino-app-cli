# Wave Generator brick

This brick provides continuous wave generation for real-time audio synthesis with multiple waveform types and smooth transitions.

## Overview

The Wave Generator brick allows you to:

- Generate continuous audio waveforms in real-time
- Select between different waveform types (sine, square, sawtooth, triangle)
- Control frequency and amplitude dynamically during playback
- Configure smooth transitions with attack, release, and glide (portamento) parameters
- Stream audio to speakers with minimal latency

It runs continuously in a background thread, producing audio blocks with configurable envelope parameters for professional-sounding synthesis.

## Features

- Four waveform types: sine, square, sawtooth, and triangle
- Real-time frequency and amplitude control with smooth transitions
- Configurable envelope parameters (attack, release, glide)
- Volume control support
- Custom speaker configuration support

## Prerequisites

Before using the Wave Generator brick, ensure you have the following:

- USB-C® Hub with external power supply (5V, 3A)
- USB audio device (USB speaker or USB-C → 3.5mm adapter)
- Arduino UNO Q running in Network Mode or SBC Mode (USB-C port needed for the hub)

## Code example and usage

Here is a basic example for generating a 440 Hz sine wave tone:

```python
from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_utils import App

wave_gen = WaveGenerator()
App.start_brick(wave_gen)

# Set frequency to A4 note (440 Hz)
wave_gen.frequency = 440.0

# Set amplitude to 80%
wave_gen.amplitude = 0.8

App.run()
```

You can customize the waveform type and envelope parameters:

```python
import time
from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_utils import App

wave_gen = WaveGenerator(
    wave_type="square",
    attack=0.01,
    release=0.03,
    glide=0.02
)
App.start_brick(wave_gen)

time.sleep(3)

# Change waveform and envelope parameters during playback
wave_gen.wave_type = "triangle"
wave_gen.attack = 0.05
wave_gen.release = 0.1
wave_gen.glide = 0.05

App.run()
```

For specific hardware configurations, you can provide a custom Speaker instance:

```python
import numpy as np
from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_peripherals.speaker import Speaker
from arduino.app_utils import App

# Create Speaker with optimal real-time configuration
speaker = Speaker(
    device=Speaker.USB_SPEAKER_1,
    sample_rate=Speaker.RATE_48K,
    channels=Speaker.CHANNELS_MONO,
    format=np.float32,
    buffer_size=Speaker.BUFFER_SIZE_REALTIME,
)

wave_gen = WaveGenerator(speaker=speaker)
wave_gen.frequency = 440.0
wave_gen.amplitude = 0.7

App.run()
```

## Understanding Wave Generation

The Wave Generator brick produces audio through continuous waveform synthesis.

The `frequency` parameter controls the pitch of the output sound, measured in Hertz (Hz), where typical audible frequencies range from 20 Hz to 8000 Hz.

The `amplitude` parameter controls the volume as a value between 0.0 (silent) and 1.0 (maximum), with smooth transitions handled by the attack and release envelope parameters.

The `attack` parameter defines how long it takes for the signal to rise from zero to its peak amplitude. A shorter attack time creates a more immediate, percussive sound, while a longer attack time produces a gradual, softer onset.

The `release` parameter defines how long the signal takes to decay from the sustain level back to zero after a note is released. This parameter is important for shaping the tail end of a sound.

The `glide` parameter (also known as portamento) smoothly transitions between frequencies over time, creating sliding pitch effects similar to a theremin or synthesizer. Setting glide to 0 disables this effect but may cause audible clicks during fast frequency changes.

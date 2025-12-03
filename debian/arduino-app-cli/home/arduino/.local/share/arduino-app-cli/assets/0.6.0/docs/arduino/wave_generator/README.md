# Wave Generator brick

This brick provides continuous wave generation for real-time audio synthesis with multiple waveform types and smooth transitions.

## Overview

The Wave Generator brick allows you to:

- Generate continuous audio waveforms in real-time
- Select between different waveform types (sine, square, sawtooth, triangle)
- Control frequency and amplitude dynamically during playback
- Configure smooth transitions with attack, release, and glide (portamento) parameters
- Stream audio to USB speakers with minimal latency

It runs continuously in a background thread, producing audio blocks at a steady rate with configurable envelope parameters for professional-sounding synthesis.

## Features

- Four waveform types: sine, square, sawtooth, and triangle
- Real-time frequency and amplitude control with smooth transitions
- Configurable envelope parameters (attack, release, glide)
- Hardware volume control support
- Thread-safe operation for concurrent access
- Efficient audio generation using NumPy vectorization
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
wave_gen.set_frequency(440.0)

# Set amplitude to 80%
wave_gen.set_amplitude(0.8)

App.run()
```

You can customize the waveform type and envelope parameters:

```python
wave_gen = WaveGenerator(
    wave_type="square",
    attack=0.01,
    release=0.03,
    glide=0.02
)

App.start_brick(wave_gen)

# Change waveform during playback
wave_gen.set_wave_type("triangle")

# Adjust envelope parameters
wave_gen.set_envelope_params(attack=0.05, release=0.1, glide=0.05)

App.run()
```

For specific hardware configurations, you can provide a custom Speaker instance:

```python
from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_peripherals.speaker import Speaker
from arduino.app_utils import App

# Create Speaker with optimal real-time configuration
speaker = Speaker(
    device=Speaker.USB_SPEAKER_2,
    sample_rate=16000,
    channels=1,
    format="FLOAT_LE",
    periodsize=480,  # 16000 Hz × 0.03s = 480 frames (eliminates buffer mismatch)
    queue_maxsize=10  # Low latency configuration
)

# Start external Speaker manually (WaveGenerator won't manage its lifecycle)
speaker.start()

wave_gen = WaveGenerator(sample_rate=16000, speaker=speaker)

App.start_brick(wave_gen)
wave_gen.set_frequency(440.0)
wave_gen.set_amplitude(0.7)

App.run()

# Stop external Speaker manually
speaker.stop()
```

**Note:** When providing an external Speaker, you manage its lifecycle (start/stop). WaveGenerator only validates configuration and uses it for playback.

## Understanding Wave Generation

The Wave Generator brick produces audio through continuous waveform synthesis.

The `frequency` parameter controls the pitch of the output sound, measured in Hertz (Hz), where typical audible frequencies range from 20 Hz to 8000 Hz.

The `amplitude` parameter controls the volume as a value between 0.0 (silent) and 1.0 (maximum), with smooth transitions handled by the attack and release envelope parameters.

The `glide` parameter (also known as portamento) smoothly transitions between frequencies over time, creating sliding pitch effects similar to a theremin or synthesizer. Setting glide to 0 disables this effect but may cause audible clicks during fast frequency changes.

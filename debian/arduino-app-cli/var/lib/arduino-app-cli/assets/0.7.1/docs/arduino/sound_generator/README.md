# Sound Generator Brick

## Overview

Sound Generator is a lightweight and expressive audio generation brick that lets you create, manipulate, and play sounds programmatically. You can write musical notes, generate tones, and compose melodies — all while shaping the sound through custom waveforms and effects.

## Features

- Generate tones and melodies from notes or frequencies
- Choose your waveform — sine, square, triangle, sawtooth
- Add sound effects such as chorus, overdrive, delay, vibrato, or distortion
- Compose procedural music directly from code
- Real-time playback over speaker

## Code example and usage

```python
from arduino.app_bricks.sound_generator import SoundGenerator, SoundEffect
from arduino.app_utils import App

player = SoundGenerator(sound_effects=[SoundEffect.adsr()])

fur_elise = [
    ("E5", 1/4), ("D#5", 1/4), ("E5", 1/4), ("D#5", 1/4), ("E5", 1/4),
    ("B4", 1/4), ("D5", 1/4),  ("C5", 1/4),  ("A4", 1/2),

    ("C4", 1/4), ("E4", 1/4),  ("A4", 1/4),  ("B4", 1/2),
    ("E4", 1/4), ("G#4", 1/4), ("B4", 1/4),  ("C5", 1/2),

    ("E4", 1/4), ("E5", 1/4),  ("D#5", 1/4), ("E5", 1/4), ("D#5", 1/4), ("E5", 1/4),
    ("B4", 1/4), ("D5", 1/4),  ("C5", 1/4),  ("A4", 1/2),

    ("C4", 1/4), ("E4", 1/4),  ("A4", 1/4),  ("B4", 1/2),
    ("E4", 1/4), ("C5", 1/4),  ("B4", 1/4),  ("A4", 1.0),
]
for note, duration in fur_elise:
    player.play(note, duration)

App.run()
```

waveform can be customized to change effect. For example, for a retro-gaming sound, you can configure "square" wave form.

```python
player = SoundGenerator(wave_form="square")
```

instead, to have a more "rock" like sound, you can add effects like:

```python
player = SoundGenerator(sound_effects=[SoundEffect.adsr(), SoundEffect.overdrive(drive=180.0), SoundEffect.chorus(depth_ms=15, rate_hz=0.2, mix=0.4)])
```

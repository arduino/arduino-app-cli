# wave_generator API Reference

## Index

- Class `WaveGenerator`

---

## `WaveGenerator` class

```python
class WaveGenerator(speaker: BaseSpeaker | None, wave_type: WaveType, attack: float, release: float, glide: float)
```

Continuous wave generator brick for audio synthesis.

This brick generates continuous audio waveforms (sine, square, sawtooth, triangle)
and streams them to a Speaker in real-time. It provides smooth transitions
between frequency and amplitude changes using configurable envelope parameters.

The generator runs continuously in a background thread, producing audio blocks
with minimal latency.

### Properties

#### `wave_type: WaveType`

Access: read/write

Get or set the current waveform type.

##### Parameters

- **wave_type** (*WaveType*): One of "sine", "square", "sawtooth", "triangle".

##### Returns

- (*WaveType*): Current waveform type ("sine", "square", "sawtooth", "triangle").

#### `sample_rate: int`

Access: read-only

Get the audio sample rate in Hz.

##### Returns

- (*int*): Sample rate in Hz.

##### Raises

- **RuntimeError**: If no speaker is configured.

#### `block_duration: float`

Access: read-only

Get the duration of each audio block in seconds.

##### Returns

- (*float*): Block duration in seconds.

#### `frequency: float`

Access: read/write

Get or set the current output frequency in Hz.

The frequency will smoothly transition to the new value over the
configured glide time.

##### Parameters

- **frequency** (*float*): Target frequency in Hz (typically 20-8000 Hz).

##### Returns

- (*float*): Current output frequency in Hz.

##### Raises

- **ValueError**: If the frequency is negative.

#### `amplitude: float`

Access: read/write

Get or set the current output amplitude.

The amplitude will smoothly transition to the new value over the
configured attack/release time.

##### Parameters

- **amplitude** (*float*): Target amplitude in range [0.0, 1.0].

##### Returns

- (*float*): Current output amplitude (0.0-1.0).

##### Raises

- **ValueError**: If the amplitude is not in range [0.0, 1.0].

#### `attack: float`

Access: read/write

Get or set the current attack time in seconds.

Attack time controls how quickly the amplitude rises to the target value.

##### Parameters

- **attack** (*float*): Attack time in seconds.

##### Returns

- (*float*): Current attack time in seconds.

##### Raises

- **ValueError**: If the attack time is negative.

#### `release: float`

Access: read/write

Get or set the current release time in seconds.

Release time controls how quickly the amplitude falls to the target value.

##### Parameters

- **release** (*float*): Release time in seconds.

##### Returns

- (*float*): Current release time in seconds.

##### Raises

- **ValueError**: If the release time is negative.

#### `glide: float`

Access: read/write

Get the current frequency glide time in seconds (portamento).

Glide time controls how quickly the frequency transitions to the target value.

##### Parameters

- **glide** (*float*): Frequency glide time in seconds.

##### Returns

- (*float*): Current frequency glide time in seconds.

##### Raises

- **ValueError**: If the glide time is negative.

#### `volume: int | None`

Access: read/write

Get or set the wave generator volume level.

##### Parameters

- **volume** (*int*): Hardware volume level (0-100).

##### Returns

- (*int*): Current volume level (0-100).

##### Raises

- **ValueError**: If the volume is not in range [0, 100].

#### `state: dict`

Access: read-only

Get current generator state.

##### Returns

- (*dict*): Dictionary containing current frequency, amplitude, wave type, etc.

### Methods

#### `start()`

Start the wave generator and audio output.

This starts the speaker device too.

#### `stop()`

Stop the wave generator and audio output.

This stops the speaker device too.


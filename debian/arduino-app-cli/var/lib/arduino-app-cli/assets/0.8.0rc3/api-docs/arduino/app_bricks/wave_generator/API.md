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

### Methods

#### `wave_type()`

Get or set the current waveform type.

##### Parameters

- **wave_type** (*WaveType*): One of "sine", "square", "sawtooth", "triangle".

##### Returns

- (*WaveType*): Current waveform type ("sine", "square", "sawtooth", "triangle").

#### `sample_rate()`

Get the audio sample rate in Hz.

##### Returns

- (*int*): Sample rate in Hz.

##### Raises

- **RuntimeError**: If no speaker is configured.

#### `block_duration()`

Get the duration of each audio block in seconds.

##### Returns

- (*float*): Block duration in seconds.

#### `frequency()`

Get or set the current output frequency in Hz.

The frequency will smoothly transition to the new value over the
configured glide time.

##### Parameters

- **frequency** (*float*): Target frequency in Hz (typically 20-8000 Hz).

##### Returns

- (*float*): Current output frequency in Hz.

##### Raises

- **ValueError**: If the frequency is negative.

#### `amplitude()`

Get or set the current output amplitude.

The amplitude will smoothly transition to the new value over the
configured attack/release time.

##### Parameters

- **amplitude** (*float*): Target amplitude in range [0.0, 1.0].

##### Returns

- (*float*): Current output amplitude (0.0-1.0).

##### Raises

- **ValueError**: If the amplitude is not in range [0.0, 1.0].

#### `attack()`

Get or set the current attack time in seconds.

Attack time controls how quickly the amplitude rises to the target value.

##### Parameters

- **attack** (*float*): Attack time in seconds.

##### Returns

- (*float*): Current attack time in seconds.

##### Raises

- **ValueError**: If the attack time is negative.

#### `release()`

Get or set the current release time in seconds.

Release time controls how quickly the amplitude falls to the target value.

##### Parameters

- **release** (*float*): Release time in seconds.

##### Returns

- (*float*): Current release time in seconds.

##### Raises

- **ValueError**: If the release time is negative.

#### `glide()`

Get the current frequency glide time in seconds (portamento).

Glide time controls how quickly the frequency transitions to the target value.

##### Parameters

- **glide** (*float*): Frequency glide time in seconds.

##### Returns

- (*float*): Current frequency glide time in seconds.

##### Raises

- **ValueError**: If the glide time is negative.

#### `volume()`

Get or set the wave generator volume level.

##### Parameters

- **volume** (*int*): Hardware volume level (0-100).

##### Returns

- (*int*): Current volume level (0-100).

##### Raises

- **ValueError**: If the volume is not in range [0, 100].

#### `state()`

Get current generator state.

##### Returns

- (*dict*): Dictionary containing current frequency, amplitude, wave type, etc.

#### `start()`

Start the wave generator and audio output.

This starts the speaker device too.

#### `stop()`

Stop the wave generator and audio output.

This stops the speaker device too.


# speaker API Reference

## Index

- Class `SpeakerException`
- Class `Speaker`

---

## `SpeakerException` class

```python
class SpeakerException()
```

Custom exception for Speaker errors.


---

## `Speaker` class

```python
class Speaker(device: str, sample_rate: int, channels: int, format: str)
```

Speaker class for reproducing audio using ALSA PCM interface.

### Parameters

- **device** (*str*): ALSA device name or USB_SPEAKER_1/2 macro.
- **sample_rate** (*int*): Sample rate in Hz (default: 16000).
- **channels** (*int*): Number of audio channels (default: 1).
- **format** (*str*): Audio format (default: "S16_LE").

### Raises

- **SpeakerException**: If the speaker cannot be initialized or if the device is busy.

### Methods

#### `list_usb_devices()`

Return a list of available USB speaker ALSA device names (plughw only).

##### Returns

- (*list*): List of USB speaker device names.

#### `get_volume()`

Get the current volume level of the speaker.

##### Returns

- (*int*): Volume level (0-100). If no mixer is available, returns -1.

##### Raises

- **SpeakerException**: If the mixer is not available or if volume cannot be retrieved.

#### `set_volume(volume: int)`

Set the volume level of the speaker.

##### Parameters

- **volume** (*int*): Volume level (0-100).

##### Raises

- **SpeakerException**: If the mixer is not available or if volume cannot be set.

#### `start()`

Start the spaker stream by opening the PCM device.

#### `stop()`

Close the PCM device if open.

#### `play(data: bytes | np.ndarray, block_on_queue: bool)`

Play audio data through the speaker.

##### Parameters

- **data** (*bytes|np.ndarray*): Audio data to play as bytes or np.ndarray.
- **block_on_queue** (*bool*): If True, block until the queue has space for the data.

##### Raises

- **SpeakerException**: If the speaker is not started or if playback fails.


# microphone API Reference

## Index

- Class `MicrophoneException`
- Class `MicrophoneDisconnectedException`
- Class `Microphone`

---

## `MicrophoneException` class

```python
class MicrophoneException()
```

Custom exception for Microphone errors.


---

## `MicrophoneDisconnectedException` class

```python
class MicrophoneDisconnectedException()
```

Raised when the microphone device is disconnected and max retries are exceeded.


---

## `Microphone` class

```python
class Microphone(device: str, sample_rate: int, channels: int, format: str, periodsize: int, max_reconnect_attempts: int, reconnect_delay: float)
```

Microphone class for capturing audio using ALSA PCM interface.

Handles automatic reconnection on device disconnection.

### Parameters

- **device** (*str*): ALSA device name or USB_MIC_1/2 macro.
- **sample_rate** (*int*): Sample rate in Hz (default: 16000).
- **channels** (*int*): Number of audio channels (default: 1).
- **format** (*str*): Audio format (default: "S16_LE").
- **periodsize** (*int*): Period size in frames (default: 1024).
- **max_reconnect_attempts** (*int*): Maximum attempts to reconnect on disconnection (default: 30).
- **reconnect_delay** (*float*): Delay in seconds between reconnection attempts (default: 2.0).

### Raises

- **MicrophoneException**: If the microphone cannot be initialized or if the device is busy.

### Methods

#### `get_volume()`

Get the current volume level of the microphone.

##### Returns

- (*int*): Volume level (0-100). If no mixer is available, returns -1.

##### Raises

- **MicrophoneException**: If the mixer is not available or if volume cannot be retrieved.

#### `set_volume(volume: int)`

Set the volume level of the microphone.

##### Parameters

- **volume** (*int*): Volume level (0-100).

##### Raises

- **MicrophoneException**: If the mixer is not available or if volume cannot be set.

#### `start()`

Start the microphone stream by opening the PCM device.

#### `connect()`

Try to connect the microphone device.

#### `stream()`

Yield audio chunks from the microphone. Each chunk has periodsize samples.

- Handles automatic reconnection if the device is unplugged and replugged.
- Only one main loop, no nested loops.
- Thread safe and clean state management.
- When max reconnect attempts are reached, the generator returns (StopIteration for the caller).
- All PCM operations are protected by lock.

##### Returns

- (*np.ndarray*): Audio data as a numpy array of the correct dtype, depending on the format specified.

#### `list_usb_devices()`

Return a list of available USB microphone ALSA device names (plughw only).

##### Returns

- (*list*): List of USB microphone device names.

#### `stop()`

Close the PCM device if open.


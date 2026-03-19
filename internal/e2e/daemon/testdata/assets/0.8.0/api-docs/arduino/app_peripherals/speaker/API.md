# speaker API Reference

## Index

- Class `Speaker`
- Class `BaseSpeaker`
- Class `ALSASpeaker`
- Class `SpeakerError`
- Class `SpeakerOpenError`
- Class `SpeakerWriteError`
- Class `SpeakerConfigError`

---

## `Speaker` class

```python
class Speaker()
```

Unified Speaker class that can be configured for different speaker types.

This class serves as both a factory and a wrapper, automatically creating
the appropriate speaker implementation based on the provided configuration.

Supports:
    - ALSA Speakers (local speakers connected to the system via ALSA)

Note: constructor arguments (except those in signature) must be provided in
keyword format to forward them correctly to the specific speaker implementations.
Refer to the documentation of each speaker type for available parameters.

### Methods

#### `play_pcm(pcm_audio: np.ndarray, sample_rate: int, channels: int, format: FormatPlain | FormatPacked, device: str | int)`

Play raw PCM audio data.

##### Parameters

- **pcm_audio** (*np.ndarray*): Raw PCM audio data in ALSA PCM format.
- **sample_rate** (*int*): Sample rate in Hz.
- **channels** (*int*): Number of audio channels.
- **format** (*FormatPlain | FormatPacked*): Audio format as one of:
- Type classes: np.int16, np.float32, np.uint8
- dtype objects: np.dtype('<i2'), np.dtype('>f4')
- Strings: 'int16', '<i2', '>f4', 'float32'
- Tuple of (format, is_packed): to specify if the format is packed (e.g. 24-bit audio)
- **device** (*Union[str, int]*) (optional): Speaker device identifier. Supports:
- int | str: ALSA device ordinal index (e.g., 0, 1, "0", "1", ...)
- str: ALSA device name (e.g., "plughw:CARD=MyCard,DEV=0", "hw:0,0", "CARD=MyCard,DEV=0")
- str: ALSA device file path (e.g., "/dev/snd/by-id/usb-My-Device-00")
- str: Speaker.USB_SPEAKER_x macros
Default: Speaker.USB_SPEAKER_1 - First USB speaker available.

##### Raises

- **SpeakerOpenError**: If speaker can't be opened.
- **SpeakerWriteError**: If speaker is not started.
- **ValueError**: If pcm_audio is empty or invalid.
- **Exception**: If the underlying implementation fails to write a frame.

#### `play_wav(wav_audio: np.ndarray, device: str | int)`

Play audio from WAV format data.

Note: Only uncompressed PCM WAV files are supported.

##### Parameters

- **wav_audio** (*np.ndarray*): WAV format audio data (including header).
- **device** (*Union[str, int]*) (optional): Speaker device identifier. Supports:
- int | str: ALSA device ordinal index (e.g., 0, 1, "0", "1", ...)
- str: ALSA device name (e.g., "plughw:CARD=MyCard,DEV=0", "hw:0,0", "CARD=MyCard,DEV=0")
- str: ALSA device file path (e.g., "/dev/snd/by-id/usb-My-Device-00")
- str: Speaker.USB_SPEAKER_x macros
Default: Speaker.USB_SPEAKER_1 - First USB speaker available.

##### Raises

- **SpeakerOpenError**: If speaker can't be opened.
- **SpeakerWriteError**: If speaker is not started.
- **ValueError**: If wav_audio is empty or invalid.
- **Exception**: If the underlying implementation fails to write a frame.


---

## `BaseSpeaker` class

```python
class BaseSpeaker(sample_rate: int, channels: int, format: FormatPlain | FormatPacked, buffer_size: int, auto_reconnect: bool)
```

Abstract base class for speaker implementations.

This class defines the common interface that all speaker implementations must follow,
providing a unified API regardless of the underlying audio playback protocol or type.

The input is always a NumPy array with the PCM format.

### Parameters

- **sample_rate** (*int*): Sample rate in Hz.
- **channels** (*int*): Number of audio channels.
- **format** (*FormatPlain | FormatPacked*): Audio format as one of:
- Type classes: np.int16, np.float32, np.uint8
- dtype objects: np.dtype('<i2'), np.dtype('>f4')
- Strings: 'int16', '<i2', '>f4', 'float32'
- Tuple of (format, is_packed): to specify if the format is packed (e.g. 24-bit audio)
- **buffer_size** (*int*): Size of the audio buffer.
- **auto_reconnect** (*bool*) (optional): Enable automatic reconnection on failure. Default: True.

### Raises

- **SpeakerConfigError**: If the provided configuration is not valid.

### Methods

#### `volume()`

Get or set the speaker volume level.

This controls the software volume of the speaker device.

##### Parameters

- **volume** (*int*): Software volume level (0-100).

##### Returns

- (*int*): Current volume level (0-100).

##### Raises

- **ValueError**: If the volume is not valid.

#### `status()`

Read-only property for camera status.

#### `start()`

Start the speaker capture.

#### `stop()`

Stop the speaker and release resources.

#### `play(audio_chunk: np.ndarray)`

Play an audio chunk on the speaker.

##### Parameters

- **audio_chunk** (*np.ndarray*): NumPy array in PCM format.

##### Raises

- **SpeakerWriteError**: If the speaker is not started.
- **ValueError**: If audio_chunk is empty or invalid.
- **Exception**: If the underlying implementation fails to write a frame.

#### `play_pcm(pcm_audio: np.ndarray)`

Play raw PCM audio data.

##### Parameters

- **pcm_audio** (*np.ndarray*): Raw PCM audio data in PCM format.

##### Raises

- **SpeakerOpenError**: If speaker can't be opened or reopened.
- **SpeakerWriteError**: If speaker is not started.
- **ValueError**: If pcm_audio is empty or invalid.
- **Exception**: If the underlying implementation fails to write a frame.

#### `play_wav(wav_audio: np.ndarray)`

Play audio from WAV format data.

Note: Only uncompressed PCM WAV files are supported.

##### Parameters

- **wav_audio** (*np.ndarray*): WAV format audio data (including header).

##### Raises

- **SpeakerOpenError**: If speaker can't be opened or reopened.
- **SpeakerWriteError**: If speaker is not started.
- **ValueError**: If wav_audio is empty or invalid.
- **Exception**: If the underlying implementation fails to write a frame.

#### `is_started()`

Check if the speaker is started.

#### `on_status_changed(callback: Callable[[str, dict], None] | None)`

Registers or removes a callback to be triggered on speaker lifecycle events.

When a speaker status changes, the provided callback function will be invoked.
If None is provided, the callback will be removed.

##### Parameters

- **callback** (*Callable[[str, dict], None]*): A callback that will be called every time the
speaker status changes with the new status and any associated data. The status
names depend on the actual speaker implementation being used. Some common events
are:
- 'connected': The speaker has been reconnected.
- 'disconnected': The speaker has been disconnected.
- **callback** (*None*): To unregister the current callback, if any.

##### Examples

```python
def on_status(status: str, data: dict):
    print(f"Speaker is now: {status}")
    print(f"Data: {data}")
    # Here you can add your code to react to the event

speaker.on_status_changed(on_status)
```

---

## `ALSASpeaker` class

```python
class ALSASpeaker(device: str | int, sample_rate: int, channels: int, format: FormatPlain | FormatPacked, buffer_size: int, shared: bool, auto_reconnect: bool)
```

ALSA (Advanced Linux Sound Architecture) speaker implementation.

This class handles local audio playback devices on Linux systems using ALSA.

### Parameters

- **device** (*Union[str, int]*): ALSA device identifier. Can be:
- int | str: device ordinal index (e.g., 0, 1, "0", "1", ...)
- str: device name (e.g., "plughw:CARD=MyCard,DEV=0", "hw:0,0", "CARD=MyCard,DEV=0")
- str: device file path (e.g., "/dev/snd/by-id/usb-My-Device-00")
- str: Speaker.USB_SPEAKER_x macros
Default: Speaker.USB_SPEAKER_1 - First USB speaker available.
- **sample_rate** (*int*): Sample rate in Hz. Default: 16000.
- **channels** (*int*): Number of audio channels. Default: 1 (mono).
- **format** (*FormatPlain | FormatPacked*): Audio format as one of:
- Type classes: np.int16, np.float32, np.uint8
- dtype objects: np.dtype('<i2'), np.dtype('>f4')
- Strings: 'int16', '<i2', '>f4', 'float32'
- Tuple of (format, is_packed): to specify if the format is packed (e.g. 24-bit audio)
Default: np.int16 - 16-bit signed platform-endian.
- **buffer_size** (*int*): Size of the audio buffer that will be used as ALSA periodsize
parameter. Low values increase CPU usage but reduce latency. Default: 1024.
- **shared** (*bool*): ALSA device sharing mode.
- False: Opens the device in exclusive mode to provide lowest latency
    but another application will fail when this instance is using the device.
- True: Opens the device in shared mode to allow other applications to use
    it at the same time but introduces higher latency. Will fail when another
    instance is already using the device in exclusive mode.
Default: True.
- **auto_reconnect** (*bool*): Enable automatic reconnection on failure.
Default: True.
- **Note**: When shared=True, only higher buffer size values are supported due to
ALSA limitations (~2000).

### Raises

- **SpeakerConfigError**: If the format is not supported.

### Methods

#### `alsa_format_idx()`

Get the ALSA format index corresponding to the current numpy dtype format.

#### `alsa_format_name()`

Get the ALSA format string corresponding to the current numpy dtype format.

#### `list_devices()`

Return a list of available ALSA speakers (plughw only).

##### Returns

- (*list*): List of speakers in ALSA device name format.

#### `list_usb_devices()`

Return a list of available USB ALSA speakers (plughw only).

##### Returns

- (*list*): List of USB speakers in ALSA device name format.


---

## `SpeakerError` class

```python
class SpeakerError()
```

Base exception for Speaker-related errors.


---

## `SpeakerOpenError` class

```python
class SpeakerOpenError()
```

Exception raised when the speaker cannot be opened.


---

## `SpeakerWriteError` class

```python
class SpeakerWriteError()
```

Exception raised when writing to speaker fails.


---

## `SpeakerConfigError` class

```python
class SpeakerConfigError()
```

Exception raised when speaker configuration is invalid.


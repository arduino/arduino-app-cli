# microphone API Reference

## Index

- Class `Microphone`
- Class `BaseMicrophone`
- Class `ALSAMicrophone`
- Class `WebSocketMicrophone`
- Class `MicrophoneError`
- Class `MicrophoneConfigError`
- Class `MicrophoneOpenError`
- Class `MicrophoneReadError`

---

## `Microphone` class

```python
class Microphone()
```

Unified Microphone class that can be configured for different microphone types.

This class serves as both a factory and a wrapper, automatically creating
the appropriate microphone implementation based on the provided configuration.

Supports:
    - ALSA Microphones (local microphones connected to the system via ALSA)
    - WebSocket Microphones (input audio streams via WebSocket client)

Note: constructor arguments (except those in signature) must be provided in
keyword format to forward them correctly to the specific microphone implementations.
Refer to the documentation of each microphone type for available parameters.

### Methods

#### `record_pcm(duration: float, sample_rate: int, channels: int, format: FormatPlain | FormatPacked, device: str | int)`

Record audio for a specified duration and return as raw PCM format.

##### Parameters

- **duration** (*float*): Recording duration in seconds.
- **sample_rate** (*int*): Sample rate in Hz.
- **channels** (*int*): Number of audio channels.
- **format** (*FormatPlain | FormatPacked*): Audio format as one of:
- Type classes: np.int16, np.float32, np.uint8
- dtype objects: np.dtype('<i2'), np.dtype('>f4')
- Strings: 'int16', '<i2', '>f4', 'float32'
- Tuple of (format, is_packed): to specify if the format is packed (e.g. 24-bit audio)
- **device** (*Union[str, int]*): Microphone device identifier. Supports:
- int | str: ALSA device ordinal index (e.g., 0, 1, "0", "1", ...)
- str: ALSA device name (e.g., "plughw:CARD=MyCard,DEV=0", "hw:0,0", "CARD=MyCard,DEV=0")
- str: ALSA device file path (e.g., "/dev/snd/by-id/usb-My-Device-00")
- str: Microphone.USB_MIC_x macros
- str: WebSocket URL for audio streams (e.g., "ws://0.0.0.0:8080")
Default: USB_MIC_1 - First USB microphone.

##### Returns

- (*np.ndarray*): Raw audio data in raw PCM format.

##### Raises

- **MicrophoneOpenError**: If microphone can't be opened.
- **MicrophoneReadError**: If no audio is available after multiple attempts.
- **ValueError**: If duration is not > 0.
- **Exception**: If the underlying implementation fails to read a frame.

#### `record_wav(duration: float, sample_rate: int, channels: int, format: FormatPlain | FormatPacked, device: str | int)`

Record audio for a specified duration and return as WAV format.

Note: Only uncompressed PCM WAV recordings are supported.

##### Parameters

- **duration** (*float*): Recording duration in seconds.
- **sample_rate** (*int*): Sample rate in Hz.
- **channels** (*int*): Number of audio channels.
- **format** (*FormatPlain | FormatPacked*): Audio format as one of:
- Type classes: np.int16, np.float32, np.uint8
- dtype objects: np.dtype('<i2'), np.dtype('>f4')
- Strings: 'int16', '<i2', '>f4', 'float32'
- Tuple of (format, is_packed): to specify if the format is packed (e.g. 24-bit audio)
- **device** (*Union[str, int]*) (optional): Microphone device identifier. Supports:
- int | str: ALSA device ordinal index (e.g., 0, 1, "0", "1", ...)
- str: ALSA device name (e.g., "plughw:CARD=MyCard,DEV=0", "hw:0,0", "CARD=MyCard,DEV=0")
- str: ALSA device file path (e.g., "/dev/snd/by-id/usb-My-Device-00")
- str: Microphone.USB_MIC_x macros
- str: WebSocket URL for audio streams (e.g., "ws://0.0.0.0:8080")
Default: USB_MIC_1 - First USB microphone.

##### Returns

- (*np.ndarray*): Raw audio data in WAV format as numpy array.

##### Raises

- **MicrophoneOpenError**: If microphone can't be opened.
- **MicrophoneReadError**: If no audio is available after multiple attempts.
- **ValueError**: If duration is not > 0.
- **Exception**: If the underlying implementation fails to read a frame.


---

## `BaseMicrophone` class

```python
class BaseMicrophone(sample_rate: int, channels: int, format: FormatPlain | FormatPacked, buffer_size: int, auto_reconnect: bool)
```

Abstract base class for microphone implementations.

This class defines the common interface that all microphone implementations must follow,
providing a unified API regardless of the underlying audio capture protocol or type.

The output is always a NumPy array with the ALSA PCM format.

### Parameters

- **sample_rate** (*int*): Sample rate in Hz (default: 16000).
- **channels** (*int*): Number of audio channels (default: 1).
- **format** (*FormatPlain | FormatPacked*): Audio format as one of:
- Type classes: np.int16, np.float32, np.uint8
- dtype objects: np.dtype('<i2'), np.dtype('>f4')
- Strings: 'int16', '<i2', '>f4', 'float32'
- Tuple of (format, is_packed): to specify if the format is packed (e.g. 24-bit audio)
- **buffer_size** (*int*): Size of the audio buffer.
- **auto_reconnect** (*bool*) (optional): Enable automatic reconnection on failure. Default: True.

### Methods

#### `volume()`

Get or set the microphone volume level.

This controls the software volume of the microphone device.

##### Parameters

- **volume** (*int*): Software volume level (0-100).

##### Returns

- (*int*): Current volume level (0-100).

##### Raises

- **ValueError**: If the volume is not valid.

#### `status()`

Read-only property for camera status.

#### `start()`

Start the microphone capture.

#### `stop()`

Stop the microphone and release resources.

#### `capture()`

Capture an audio chunk from the microphone.

##### Returns

-: Numpy array in ALSA PCM format or None if no audio is available.

##### Raises

- **MicrophoneReadError**: If the microphone is not started.
- **Exception**: If the underlying implementation fails to read a frame.

#### `stream()`

Continuously capture audio chunks from the microphone.

This is a generator that yields audio chunks continuously while the microphone is started.

##### Returns

- (*np.ndarray*): Audio chunks as numpy arrays.

#### `is_started()`

Check if the microphone is started.

#### `on_status_changed(callback: Callable[[str, dict], None] | None)`

Registers or removes a callback to be triggered on microphone lifecycle events.

When a microphone status changes, the provided callback function will be invoked.
If None is provided, the callback will be removed.

##### Parameters

- **callback** (*Callable[[str, dict], None]*): A callback that will be called every time the
microphone status changes with the new status and any associated data. The status
names depend on the actual microphone implementation being used. Some common events
are:
- 'connected': The microphone has been reconnected.
- 'disconnected': The microphone has been disconnected.
- 'streaming': The stream is streaming.
- 'paused': The stream has been paused and is temporarily unavailable.
- **callback** (*None*): To unregister the current callback, if any.

##### Examples

```python
def on_status(status: str, data: dict):
    print(f"Microphone is now: {status}")
    print(f"Data: {data}")
    # Here you can add your code to react to the event

microphone.on_status_changed(on_status)
```
#### `record_pcm(duration: float)`

Record audio for a specified duration and return as raw PCM format.

##### Parameters

- **duration** (*float*): Recording duration in seconds.

##### Returns

- (*np.ndarray*): Raw audio data in raw ALSA PCM format.

##### Raises

- **MicrophoneOpenError**: If microphone can't be opened or reopened.
- **MicrophoneReadError**: If no audio is available after multiple attempts.
- **ValueError**: If duration is not > 0.
- **Exception**: If the underlying implementation fails to read a frame.

#### `record_wav(duration: float)`

Record audio for a specified duration and return as WAV format.

Note: Only uncompressed PCM WAV recordings are supported.

##### Parameters

- **duration** (*float*): Recording duration in seconds.

##### Returns

- (*np.ndarray*): Raw audio data in WAV format as numpy array.

##### Raises

- **MicrophoneOpenError**: If microphone can't be opened or reopened.
- **MicrophoneReadError**: If no audio is available after multiple attempts.
- **ValueError**: If duration is not > 0.
- **Exception**: If the underlying implementation fails to read a frame.


---

## `ALSAMicrophone` class

```python
class ALSAMicrophone(device: str | int, sample_rate: int, channels: int, format: FormatPlain | FormatPacked, buffer_size: int, shared: bool, auto_reconnect: bool)
```

ALSA (Advanced Linux Sound Architecture) microphone implementation.

This class handles local audio capture devices on Linux systems using ALSA.

### Parameters

- **device** (*Union[str, int]*): ALSA device identifier. Can be:
- int | str: device ordinal index (e.g., 0, 1, "0", "1", ...)
- str: device name (e.g., "plughw:CARD=MyCard,DEV=0", "hw:0,0", "CARD=MyCard,DEV=0")
- str: device file path (e.g., "/dev/snd/by-id/usb-My-Device-00")
- str: Microphone.USB_MIC_x macros
Default: Microphone.USB_MIC_1 - First USB microphone.
- **sample_rate** (*int*): Sample rate in Hz (default: 16000).
- **channels** (*int*): Number of audio channels (default: 1).
- **format** (*FormatPlain | FormatPacked*): Audio format as one of:
- Type classes: np.int16, np.float32, np.uint8
- dtype objects: np.dtype('<i2'), np.dtype('>f4')
- Strings: 'int16', '<i2', '>f4', 'float32'
- Tuple of (format, is_packed): to specify if the format is packed (e.g. 24-bit audio)
Default: np.int16 - 16-bit signed platform-endian.
- **buffer_size** (*int*): Size of the audio buffer (default: 1024).
- **shared** (*bool*): ALSA device sharing mode.
- False: Opens the device in exclusive mode to provide lowest latency
    but another application will fail when this instance is using the device.
- True: Opens the device in shared mode to allow other applications to use
    it at the same time but introduces higher latency. Will fail when another
    instance is already using the device in exclusive mode.
Default: True.
- **auto_reconnect** (*bool*) (optional): Enable automatic reconnection on failure.
Default: True.
- **Note**: When shared=True, only higher buffer size values are supported due to
ALSA limitations (~2000).

### Raises

- **MicrophoneConfigError**: If the format is not supported.

### Methods

#### `alsa_format_idx()`

Get the ALSA format index corresponding to the current numpy dtype format.

#### `alsa_format_name()`

Get the ALSA format string corresponding to the current numpy dtype format.

#### `list_devices()`

Return a list of available ALSA microphones (plughw only).

##### Returns

- (*list*): List of speakers in ALSA device name format.

#### `list_usb_devices()`

Return a list of available USB ALSA microphones (plughw only).

##### Returns

- (*list*): List of USB microphones in ALSA device name format.


---

## `WebSocketMicrophone` class

```python
class WebSocketMicrophone(port: int, timeout: int, certs_dir_path: str, use_tls: bool, secret: str, encrypt: bool, sample_rate: int, channels: int, format: FormatPlain | FormatPacked, buffer_size: int, auto_reconnect: bool)
```

WebSocket Microphone implementation that hosts a WebSocket server.

This microphone exposes a WebSocket server that receives audio chunks from connected
clients. Only one client can be connected at a time.

Clients must encode the audio data in PCM format and serialize the content in the
binary format supported by BPPCodec.

Secure communication with the WebSocket server is supported in three security modes:
- Security disabled (empty secret)
- Authenticated (secret + encrypt=False) - HMAC-SHA256
- Authenticated + Encrypted (secret + encrypt=True) - ChaCha20-Poly1305

Also, clients are expected to respect the sample rate, channels, format, and chunk
size specified during initialization.

### Parameters

- **port** (*int*): Port to bind the server to. Default: 8080.
- **timeout** (*int*): Connection timeout in seconds. Default: 3.
- **certs_dir_path** (*str*): Path to the directory containing TLS certificates.
Default: "/app/certs".
- **use_tls** (*bool*): Enable TLS for secure connections. If True, 'encrypt' will
be ignored. Use this for transport-level security with clients that can
accept self-signed certificates or when supplying your own certificates.
Default: False.
- **secret** (*str*): Secret key for authentication/encryption (empty = security disabled).
Default: empty.
- **encrypt** (*bool*): Enable encryption (only effective if secret is provided).
Default: False.
- **sample_rate** (*int*): Sample rate in Hz. Default: 16000.
- **channels** (*int*): Number of audio channels. Default: Microphone.CHANNELS_MONO - 1.
- **format** (*FormatPlain | FormatPacked*): Audio format as one of:
- Type classes: np.int16, np.float32, np.uint8
- dtype objects: np.dtype('<i2'), np.dtype('>f4')
- Strings: 'int16', '<i2', '>f4', 'float32'
- Tuple of (format, is_packed): to specify if the format is packed (e.g. 24-bit audio)
Default: np.int16 - 16-bit signed platform-endian.
- **buffer_size** (*int*): Number of frames per buffer (default: 1024). This parameter is advisory,
it's sent to clients to suggest an optimal buffer size but clients may ignore it.
Default: Microphone.BUFFER_SIZE_BALANCED - 1024.
- **auto_reconnect** (*bool*): Enable automatic reconnection on failure.

### Methods

#### `url()`

Return the WebSocket server address.

#### `security_mode()`

Return current security mode for logging/debugging.


---

## `MicrophoneError` class

```python
class MicrophoneError()
```

Base exception for microphone-related errors.


---

## `MicrophoneConfigError` class

```python
class MicrophoneConfigError()
```

Exception raised when microphone configuration is invalid.


---

## `MicrophoneOpenError` class

```python
class MicrophoneOpenError()
```

Exception raised when the microphone cannot be opened.


---

## `MicrophoneReadError` class

```python
class MicrophoneReadError()
```

Exception raised when reading from microphone fails.


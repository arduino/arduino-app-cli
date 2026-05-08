# camera API Reference

## Index

- Class `Camera`
- Class `BaseCamera`
- Class `V4LCamera`
- Class `IPCamera`
- Class `WebSocketCamera`
- Class `CSICamera`
- Class `CameraError`
- Class `CameraConfigError`
- Class `CameraOpenError`
- Class `CameraReadError`
- Class `CameraTransformError`

---

## `Camera` class

```python
class Camera()
```

Unified Camera class that can be configured for different camera types.

This class serves as both a factory and a wrapper, automatically creating
the appropriate camera implementation based on the provided configuration.

Supports:
    - USB Cameras (local cameras connected using USB interface)
    - CSI Cameras (local cameras connected using MIPI CSI-2 interface)
    - IP Cameras (network-based cameras via RTSP, HLS)
    - WebSocket Cameras (input video streams via WebSocket client)

Note: constructor arguments (except those in signature) must be provided in
keyword format to forward them correctly to the specific camera implementations.


---

## `BaseCamera` class

```python
class BaseCamera(resolution: tuple[int, int], fps: int, adjustments: Callable[[np.ndarray], np.ndarray] | None, auto_reconnect: bool)
```

Abstract base class for camera implementations.

This class defines the common interface that all camera implementations must follow,
providing a unified API regardless of the underlying camera protocol or type.

### Parameters

- **resolution** (*tuple*) (optional): Resolution as (width, height). None uses default resolution.
- **fps** (*int*): Frames per second to capture from the camera.
- **adjustments** (*callable*) (optional): Function or function pipeline to adjust frames that takes
a numpy array and returns a numpy array. Default: None.
- **auto_reconnect** (*bool*) (optional): Enable automatic reconnection on failure. Default: True.

### Properties

#### `status: Literal['disconnected', 'connected', 'streaming', 'paused']`

Access: read-only

Read-only property for camera status.

### Methods

#### `start()`

Start the camera capture with retries, if enabled.

##### Raises

- **CameraOpenError**: If the camera fails to start after the retries.
- **Exception**: If the underlying implementation fails to start the camera.

#### `stop()`

Stop the camera and release resources.

#### `capture()`

Capture a frame from the camera, respecting the configured FPS.

##### Returns

-: Numpy array or None if no frame is available.

##### Raises

- **CameraReadError**: If the camera is not started.
- **Exception**: If the underlying implementation fails to read a frame.

#### `stream()`

Continuously capture frames from the camera.

This is a generator that yields frames continuously while the camera is started.
Built on top of capture() for convenience.

##### Returns

- (*np.ndarray*): Video frames as numpy arrays.

##### Raises

- **CameraReadError**: If the camera is not started.

#### `record(duration)`

Record video for a specified duration and return it as a numpy array of raw frames.

##### Parameters

- **duration** (*float*): Recording duration in seconds.

##### Returns

- (*np.ndarray*): numpy array of raw frames.

##### Raises

- **CameraReadError**: If camera is not started or any read error occurs.
- **ValueError**: If duration is not positive.
- **MemoryError**: If memory allocation for the full recording fails.

#### `record_avi(duration)`

Record video for a specified duration and return as MJPEG in AVI container.

##### Parameters

- **duration** (*float*): Recording duration in seconds.

##### Returns

- (*np.ndarray*): AVI file containing MJPEG video.

##### Raises

- **CameraReadError**: If camera is not started or any read error occurs.
- **ValueError**: If duration is not positive.
- **MemoryError**: If memory allocation for the full recording fails.

#### `is_started()`

Check if the camera has been started.

#### `on_status_changed(callback: Callable[[str, dict], None] | None)`

Registers or removes a callback to be triggered on camera lifecycle events.

When a camera status changes, the provided callback function will be invoked.
If None is provided, the callback will be removed.

##### Parameters

- **callback** (*Callable[[str, dict], None]*): A callback that will be called every time the
camera status changes with the new status and any associated data. The status names
depend on the actual camera implementation being used. Some common events are:
- 'connected': The camera has been reconnected.
- 'disconnected': The camera has been disconnected.
- 'streaming': The stream is streaming.
- 'paused': The stream has been paused and is temporarily unavailable.
- **callback** (*None*): To unregister the current callback, if any.

##### Examples

```python
def on_status(status: str, data: dict):
    print(f"Camera is now: {status}")
    print(f"Data: {data}")
    # Here you can add your code to react to the event

camera.on_status_changed(on_status)
```

---

## `V4LCamera` class

```python
class V4LCamera(device: str | int, resolution: tuple[int, int], fps: int, adjustments: Optional[Callable[[np.ndarray], np.ndarray]], auto_reconnect: bool, codec: Literal['', 'YUVY', 'MJPG', 'H264'])
```

V4L (Video4Linux) camera implementation for physically connected cameras.

This class handles USB cameras and other V4L-compatible devices on Linux systems.

### Parameters

- **device**: Camera identifier in the form of either:
- int: Camera ordinal index (e.g., 0, 1)
- str: Camera ordinal index as string (e.g., "0", "1")
- str: Camera device path (e.g., "/dev/video0", "/dev/v4l/by-id/...",
    "/dev/v4l/by-path/...")
Default: 0 (first available USB camera).
- **resolution** (*tuple[int, int]*): Resolution as (width, height). None uses default resolution.
- **fps** (*int*): Frames per second to capture from the camera. Default: 10.
- **adjustments** (*callable*) (optional): Function or function pipeline to adjust frames that takes
a numpy array and returns a numpy array. Default: None
- **auto_reconnect** (*bool*): Enable automatic reconnection on failure. Default: True.
- **codec** (*str*) (optional): Video codec to use (FourCC). Options: "YUVY", "MJPG", "H264".
Default: "" (auto).

### Methods

#### `list_devices()`

Return a list of available USB cameras.

##### Returns

- (*list[int]*): List of USB camera indices.


---

## `IPCamera` class

```python
class IPCamera(url: str, username: str | None, password: str | None, timeout: int, resolution: tuple[int, int], fps: int, adjustments: Callable[[np.ndarray], np.ndarray] | None, auto_reconnect: bool)
```

IP Camera implementation for network-based cameras.

Supports RTSP, HTTP, and HTTPS camera streams.
Can handle authentication and various streaming protocols.

### Parameters

- **url**: Camera stream URL (i.e. rtsp://..., http://..., https://...)
- **username**: Optional authentication username
- **password**: Optional authentication password
- **timeout**: Connection timeout in seconds
- **resolution** (*tuple*) (optional): Resolution as (width, height). None uses default resolution.
- **fps** (*int*): Frames per second to capture from the camera.
- **adjustments** (*callable*) (optional): Function or function pipeline to adjust frames that takes
a numpy array and returns a numpy array. Default: None
- **auto_reconnect** (*bool*) (optional): Enable automatic reconnection on failure. Default: True.


---

## `WebSocketCamera` class

```python
class WebSocketCamera(port: int, timeout: int, certs_dir_path: str, use_tls: bool, secret: str | None, encrypt: bool, resolution: tuple[int, int], fps: int, adjustments: Callable[[np.ndarray], np.ndarray] | None, auto_reconnect: bool)
```

WebSocket Camera implementation that hosts a WebSocket server.

This camera acts as a WebSocket server that receives frames from a connected
client. Only one client can be connected at a time.

The client must encode video frames in one of these formats:
- JPEG
- PNG
- WebP
- BMP
- TIFF

Communication uses the BPP (Binary Peripheral Protocol) in three security modes:
- Security disabled (secret=None) - BPP with no authentication
- Authenticated (secret + encrypt=False) - BPP with HMAC-SHA256
- Authenticated + Encrypted (secret + encrypt=True) - BPP with ChaCha20-Poly1305

By default, all modes use BPP framing. When security is disabled (secret=None),
clients can opt out of BPP by connecting with the "raw=true" URL query parameter,
allowing them to send raw image bytes directly without BPP wrapping. This parameter
is silently ignored when security is enabled.

When connecting, clients can specify a "client_name" parameter in the URL query string
to identify themselves. This name will be sanitized to allow only alphanumeric chars,
whitespace, hyphens, and underscores, and limit its length to 64 characters.

### Parameters

- **port** (*int*): Port to bind the server to
- **timeout** (*int*): Connection timeout in seconds
- **certs_dir_path** (*str*): Path to the directory containing TLS certificates
- **use_tls** (*bool*): Enable TLS for secure connections. If True, 'encrypt' will
be ignored. Use this for transport-level security with clients that can
accept self-signed certificates or when supplying your own certificates.
- **secret** (*str | None*): Pre-shared secret key used for HMAC-SHA256
authentication, or to derive the ChaCha20-Poly1305 key when
encrypt is True. None disables security. Default: None.
- **encrypt** (*bool*): Enable ChaCha20-Poly1305 encryption. Requires a
non-None secret; raises RuntimeError otherwise. Default: False.
- **resolution** (*tuple[int, int]*): Resolution as (width, height)
- **fps** (*int*): Frames per second to capture
- **adjustments** (*Callable[[np.ndarray], np.ndarray] | None*): Function to adjust frames
- **auto_reconnect** (*bool*): Enable automatic reconnection on failure

### Properties

#### `url: str`

Access: read-only

Return the WebSocket server address.

#### `security_mode: str`

Access: read-only

Return current security mode for logging/debugging.


---

## `CSICamera` class

```python
class CSICamera(device: str | int, resolution: tuple[int, int], fps: int, adjustments: Optional[Callable[[np.ndarray], np.ndarray]], auto_reconnect: bool)
```

CSI (Camera Serial Interface) camera implementation for physically connected cameras.

This class handles CSI cameras on Linux systems.

### Parameters

- **device**: Camera identifier in the form of either:
- int: Camera ordinal index (e.g., 0, 1)
- str: Camera ordinal index as string (e.g., "0", "1")
- str: Camera name (e.g., "CAMERA0", "CAMERA1")
Default: 0 (first available CSI camera).
- **resolution** (*tuple*) (optional): Resolution as (width, height). None uses default resolution.
- **fps** (*int*) (optional): Frames per second to capture from the camera. Default: 10.
- **adjustments** (*callable*) (optional): Function or function pipeline to adjust frames that takes
a numpy array and returns a numpy array. Default: None.
- **auto_reconnect** (*bool*) (optional): Enable automatic reconnection on failure. Default: True.

### Methods

#### `list_devices()`

Return a sorted list of available CSI cameras.

##### Returns

- (*list[int]*): List of CSI camera indices.

#### `list_device_names()`

Return a list of available CSI cameras.

##### Returns

- (*list[str]*): List of CSI camera device paths.


---

## `CameraError` class

```python
class CameraError()
```

Base exception for camera-related errors.


---

## `CameraConfigError` class

```python
class CameraConfigError()
```

Exception raised when camera configuration is invalid.


---

## `CameraOpenError` class

```python
class CameraOpenError()
```

Exception raised when the camera cannot be opened.


---

## `CameraReadError` class

```python
class CameraReadError()
```

Exception raised when reading from camera fails.


---

## `CameraTransformError` class

```python
class CameraTransformError()
```

Exception raised when frame transformation fails.


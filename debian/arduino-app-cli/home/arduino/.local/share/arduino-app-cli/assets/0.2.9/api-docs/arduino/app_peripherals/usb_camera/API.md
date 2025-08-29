# usb_camera API Reference

## Index

- Class `CameraReadError`
- Class `CameraOpenError`
- Class `USBCamera`

---

## `CameraReadError` class

```python
class CameraReadError()
```

Exception raised when the specified camera cannot be found.


---

## `CameraOpenError` class

```python
class CameraOpenError()
```

Exception raised when the camera cannot be opened.


---

## `USBCamera` class

```python
class USBCamera(camera: int, resolution: tuple[int, int], fps: int, compression: bool, letterbox: bool)
```

Represents an input peripheral for capturing images from a USB camera device.

This class uses OpenCV to interface with the camera and capture images.

### Parameters

- **camera** (*int*): Camera index (default is 0 - index is related to the first camera available from /dev/v4l/by-id devices).
- **resolution** (*tuple[int, int]*): Resolution as (width, height). If None, uses default resolution.
- **fps** (*int*): Frames per second for the camera. If None, uses default FPS.
- **compression** (*bool*): Whether to compress the captured images. If True, images are compressed to PNG format.
- **letterbox** (*bool*): Whether to apply letterboxing to the captured images.

### Methods

#### `capture()`

Captures a frame from the camera, blocking to respect the configured FPS.

##### Returns

-: PIL.Image.Image | None: The captured frame as a PIL Image, or None if no frame is available.

#### `capture_bytes()`

Captures a frame from the camera and returns its raw bytes, blocking to respect the configured FPS.

##### Returns

-: bytes | None: The captured frame as a bytes array, or None if no frame is available.

#### `start()`

Starts the camera capture.

#### `stop()`

Stops the camera and releases its resources.

#### `produce()`

Alias for capture method.


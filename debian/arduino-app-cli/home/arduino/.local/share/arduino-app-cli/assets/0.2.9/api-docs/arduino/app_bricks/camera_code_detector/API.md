# camera_code_detector API Reference

## Index

- Class `CameraCodeDetector`
- Class `CameraMultiCodeDetector`
- Class `Detection`
- Function `utils.draw_bounding_box`

---

## `CameraCodeDetector` class

```python
class CameraCodeDetector(camera: USBCamera, detect_qr: bool, detect_barcode: bool)
```

Module for detecting a QR code and/or a traditional barcode using a USB camera feed.

### Methods

#### `on_detect(callback: Callable[[Image, Detection], None] | None)`

Registers or removes a callback to be triggered on code detection.

When a QR code or barcode is detected in the camera feed, the provided callback function will be invoked. The callback function should accept the Image frame and a Detection object. If None is provided, the callback is removed.

##### Parameters

- **callback** (*Callable[[Image, Detection], None]*): A callback that will be called every time a detection is made.
- **callback** (*None*): To unregister the current callback, if any.

##### Examples

```python
def on_code_detected(frame: Image, detection: Detection):
    print(f"Detected {detection.type} with content: {detection.content}")
    # Here you can add your code to process the detected code,
    # e.g., draw a bounding box, save it to a database or log it.

detector.on_detect(on_code_detected)
```

---

## `CameraMultiCodeDetector` class

```python
class CameraMultiCodeDetector(camera: USBCamera, detect_qr: bool, detect_barcode: bool)
```

Module for detecting multiple QR codes and/or traditional barcodes using a USB camera feed.

### Methods

#### `on_detect(callback: Callable[[Image, list[Detection]], None] | None)`

Registers or removes a callback to be triggered when one or more codes are detected.

The callback is invoked for each processed video frame that contains at least one detection. The callback function should accept the Image frame and a list of Detection objects, which may include multiple QR codes and/or barcodes. If None is provided, the callback is removed.

##### Parameters

- **callback** (*Callable[[Image, list[Detection]], None] | None*): A function to be called with the current frame and all detections found in it
- **callback** (*None*): To unregister the current callback, if any.

##### Examples

```python
def on_codes_detected(frame: Image, detections: list[Detection]):
    print(f"Detected {len(detections)} codes")
    # Here you can add your code to process the detected codes,
    # e.g., draw bounding boxes, save them to a database or log them.
    
detector.on_detect(on_codes_detected)
```

---

## `Detection` class

```python
class Detection(content: str, type: str, coords: np.ndarray)
```

This class represents a single QR code or barcode detection result from a video frame.

This data structure holds the decoded content, the type of code, and its location
in the image as determined by the detection algorithm.

### Attributes

- **content** (*str*): The decoded string extracted from the QR code or barcode.
- **type** (*str*): The type of code detected, typically "QRCODE" or "BARCODE".
- **coords** (*np.ndarray*): A NumPy array of shape (4, 2) representing the four corner
points (x, y) of the detected code region in the image.


---

## `utils.draw_bounding_box` function

```python
def draw_bounding_box(frame: Image, detection: Detection)
```

Draws a bounding box and label on an image for a detected QR code or barcode.

This function overlays a green polygon around the detected code area and
adds a text label above (or below) the bounding box with the code type and content.

### Parameters

- **frame** (*Image*): The PIL Image object to draw on. This image will be modified in-place.
- **detection** (*Detection*): The detection result containing the code's content, type, and corner coordinates.

### Returns

- (*Image*): The annotated image with a bounding box and label drawn.


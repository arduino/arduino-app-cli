# Edge Impulse Model Specification

This specification defines the Edge Impulse AI model format used with Arduino Apps via AI Bricks.

Edge Impulse models are distributed as `.eim` binaries: .eim files are self-contained, platform-specific artifacts that bundle the model weights and inference runtime together. For more details, refer to the [Edge Impulse documentation](https://edgeimpulse.com).

In the Arduino Apps context, the `arduino-app-cli` recognizes an Edge Impulse model as a directory containing two required files: a descriptor file (`model.yaml`) and the model asset (`.eim` file).

EI Models can be added to the board by the user, either downloaded from [Edge Impulse](https://www.edgeimpulse.com/) via App Lab or imported manually. They are stored in a dedicated user-writable folder.

## Understanding Bricks and AI Bricks

A **Brick** is a modular service that acts as a standardized interface for specific functionalities. An **AI Brick** is a specialized Brick that manages a specific AI domain or use case (e.g., _Object Detection_, _Speech-to-Text_, or _Face Detection_). For a general introduction to Bricks and how they work, see [Understanding Bricks](https://docs.arduino.cc/software/app-lab/bricks/about-bricks/).

## EI Model Brick Relationship and Compatibility

The ecosystem implements a flexible N:N (many-to-many) relationship between Models and Bricks. Compatibility is determined by the model's category as defined in Edge Impulse: for example, a model in the "Object Detection" category is automatically compatible with both `arduino:object_detection` and `arduino:video_object_detection` bricks.

Conversely, a Brick can support multiple models of the same category, allowing users to swap models (e.g., from a lightweight to a more accurate one) while keeping the Python API identical.

A single EI Model can also be shared simultaneously by multiple Bricks. In that case, the model assets are shared, but a separate AI Runner instance is created for each Brick.

## Model Lifecycle and States in Arduino App Lab

In App Lab a model can have the following states:

- **Available**: The model is available in the user's [Edge Impulse projects](https://studio.edgeimpulse.com) but has not been downloaded to the board yet.
- **Installed**: The `.eim` file and the `model.yaml` descriptor are present on the board and ready for execution.
- **Detached**: An "Installed" model whose link to the original remote Edge Impulse project has been severed (e.g., the user logged out, or the project was deleted from the cloud). The model remains functional locally but cannot receive updates.

## Model folder

The `model.yaml` file is the manifest that describes an Edge Impulse model. It is the single source of truth for identifying the model's capabilities and requirements. Only the `.yaml` extension is supported.
The descriptor is mandatory and must be placed at the root of the model directory, alongside the corresponding `.eim` file. The model folder itself must be placed under the custom models path (see the [Custom models path](#custom-models-path) section).

## model.yaml file format

The `model.yaml` file uses a structured format to define the model's identity, its execution environment, and its compatibility with various Bricks.

- **`id`** (String, Mandatory): A string that uniquely identifies the model within the board (e.g., a UUID or a slug).
- **`name`** (String, Mandatory): A human-readable name displayed in Arduino App Lab.
- **`runner`** (String, Mandatory): Specifies the **Runner** (always `brick`).
- **`description`** (String, Optional): A brief summary of the model's purpose and capabilities.
- **`category`** (String, Optional): The functional domain of the model (e.g., `Images`, `Audio`, `Text`).
- **`bricks`** (List[Brick], Mandatory): A list of **AI Bricks** compatible with this model.
- **`metadata`** (Map, Mandatory): A dictionary of key-value pairs for additional provider-specific information. Supports several types such as strings, integers, and booleans.

### Metadata Fields

The following metadata fields are required for Edge Impulse models:

- **`source`** (String, Mandatory): Must be set to `"edgeimpulse"`.
- **`ei-project-id`** (string, Mandatory): The numeric ID of the Edge Impulse project.
- **`ei-impulse-id`** (string, Mandatory): The numeric ID of the impulse within the project.
- **`ei-impulse-name`** (String, Mandatory): The human-readable name of the impulse.
- **`ei-model-type`** (String, Mandatory): The model type. Must be `float32`.
- **`ei-engine`** (String, Mandatory): The inference engine. Must be `tflite`.
- **`ei-deployment-version`** (string, Mandatory): The deployment version number from Edge Impulse.
- **`ei-last-modified`** (String, Optional): Timestamp of the last project modification in RFC3339Nano format.
- **`ei-model-url`** (String, Optional): URL to the model page on Edge Impulse Studio.
- **`ei-gpu-mode`** (Boolean, Optional): Whether the model runs in GPU mode. Defaults to `false`.

### Brick Configuration

For Edge Impulse compatible bricks, the `bricks` list defines which AI Bricks are compatible with the model and how they are configured. The `model_configuration` for each brick must include two variables:

- **`EI_*_MODEL`** (String, Mandatory): The absolute path to the `.eim` binary. The variable name follows the pattern `EI_<BRICK_TYPE>_MODEL` (e.g.,
  `EI_OBJ_DETECTION_MODEL`, `EI_CLASSIFICATION_MODEL`). The exact name depends on the target brick (see the "Edge Impulse Category to AI Brick Mapping" section).
- **`CUSTOM_MODEL_PATH`** (String, Mandatory): The absolute path to the model's root directory (the directory containing `model.yaml`).

### Edge Impulse Category to AI Brick Mapping

The compatible bricks are determined by the **project category** set in Edge Impulse Studio. The table lists the mapping between Edge Impulse project categories and the corresponding AI Brick IDs, along with the required `model_configuration` variable name for each brick.

| EI Project Category     | AI Brick ID                           | `model_configuration` variable         |
| ----------------------- | ------------------------------------- | -------------------------------------- |
| Object Detection        | `arduino:object_detection`            | `EI_OBJ_DETECTION_MODEL`               |
| Object Detection        | `arduino:video_object_detection`      | `EI_V_OBJ_DETECTION_MODEL`             |
| Images (Classification) | `arduino:image_classification`        | `EI_CLASSIFICATION_MODEL`              |
| Images (Classification) | `arduino:video_image_classification`  | `EI_V_CLASSIFICATION_MODEL`            |
| Images (Visual Anomaly) | `arduino:visual_anomaly_detection`    | `EI_V_ANOMALY_DETECTION_MODEL`         |
| Audio                   | `arduino:audio_classification`        | `EI_AUDIO_CLASSIFICATION_MODEL`        |
| Keyword Spotting        | `arduino:audio_classification`        | `EI_AUDIO_CLASSIFICATION_MODEL`        |
| Keyword Spotting        | `arduino:keyword_spotting`            | `EI_KEYWORD_SPOTTING_MODEL`            |
| Accelerometer           | `arduino:motion_detection`            | `EI_MOTION_DETECTION_MODEL`            |
| Accelerometer           | `arduino:vibration_anomaly_detection` | `EI_VIBRATION_ANOMALY_DETECTION_MODEL` |

> **Note on Visual Anomaly Detection**: The `arduino:visual_anomaly_detection` brick is only compatible with impulses that include a `KerasVisualAnomaly` learn block. Generic image classification impulses from the `Images` category are not compatible with this brick.

### Complete Example - Object Detection Model (`model.yaml`)

```yaml
id: ei-model-842271-1
name: Hand gestures
runner: brick
description: >-
  A lightweight vision model that detects four hand gestures: neutral fist,
  open hand (five), V-sign (peace), and thumbs-up (good).
category: Images
bricks:
  - id: arduino:object_detection
    model_configuration:
      EI_OBJ_DETECTION_MODEL: /home/arduino/.arduino-bricks/models/custom-ei/ei-model-842271-1/model.eim
      CUSTOM_MODEL_PATH: /home/arduino/.arduino-bricks/models/custom-ei/ei-model-842271-1
  - id: arduino:video_object_detection
    model_configuration:
      EI_V_OBJ_DETECTION_MODEL: /home/arduino/.arduino-bricks/models/custom-ei/ei-model-842271-1/model.eim
      CUSTOM_MODEL_PATH: /home/arduino/.arduino-bricks/models/custom-ei/ei-model-842271-1
metadata:
  source: edgeimpulse
  ei-project-id: "842271"
  ei-impulse-id: "1"
  ei-impulse-name: "Hand gestures impulse"
  ei-last-modified: "2026-04-03T15:35:35.214Z"
  ei-model-type: float32
  ei-engine: tflite
  ei-deployment-version: "3"
  ei-model-url: https://studio.edgeimpulse.com/public/842271/live
  ei-gpu-mode: false
```

### Custom models path

The default location for user-installed models is `/home/<username>/.arduino-bricks/models/`, but it can be overridden via environment variable `ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR`.

The system recursively scans the root directory for valid model folders. Folder names and nesting levels are flexible, with one constraint: **the scanner stops descending into a folder as soon as it finds a `model.yaml`**. This means a model cannot be nested inside another model's folder.

```text
/models/
├── custom-ei/                  # Example: Grouped by provider
│   └── impulse-123/            # A valid model folder
│       ├── model.yaml          # Mandatory metadata
│       ├── README.md           # Optional documentation for App Lab
│       └── model.eim           # Provider-specific asset
├── nested-independent/         # Example: Independent nested structure
│   └── ei-model-456/           # A valid model folder
│       ├── model.yaml
│       └── model.eim
└── direct-model-folder/        # Example: Flat structure
    ├── model.yaml
    └── my-model.eim
```

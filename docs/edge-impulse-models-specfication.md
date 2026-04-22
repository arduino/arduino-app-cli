# Edge Impulse Model Specification



This is the specification for the Edge Impulse AI model format, to be used with Arduino Apps via AI Bricks.

Edge Impulse models are distributed as `.eim` binaries. Each `.eim` file is a self-contained, platform-specific binary that bundles the model weights and the inference runtime (check [Edge Impulse](https://edgeimpulse.com) for more information).

An Edge Impulse Model is defined by a directory (root folder) containing its descriptor and the actual model asset (.eim file). To ensure portability and allow the `arduino-app-cli` to manage models effectively, the internal structure must follow specific conventions: the model descriptor must be named `model.yaml` and placed in the root of the model folder (see "The Model Descriptor" section), and the model must be stored in the designated directory (see "Storage Hierarchy" section).

EI Models could be added to the board by the user. These can be downloaded from [Edge Impulse](https://www.edgeimpulse.com/) via App Lab or manually imported. They are stored in a dedicated user-writable partition.

## Understanding Bricks and AI Bricks

A **Brick** is a modular service that acts as a standardized interface for specific functionalities. An **AI Brick** is a specialized Brick that manages a specific AI domain or use case (e.g., _Object Detection_, _Speech-to-Text_, or _Face Detection_). For a general introduction to Bricks and how they work, see [Understanding Bricks](https://docs.arduino.cc/software/app-lab/bricks/about-bricks/).

## EI Model-AI Brick Relationship and Compatibility

The ecosystem implements a flexible N:N (Many-to-Many) relationship between Bricks and Models. A single EI Model can be compatible with multiple Bricks: compatibility is determined by the model's category as defined by Edge Impulse. For example, a model belonging to the "Object Detection" category is automatically compatible with both arduino:object_detection and arduino:video_object_detection bricks.
Conversely, a Brick can support multiple Models of the same class, allowing users to swap models (for instance, switching from a lightweight model to a more accurate one) while keeping the Python API identical. A single EI model can also be shared and utilized simultaneously by multiple Bricks: in that scenario, the model assets are shared, but a separate AI Runner instance is created for each Brick.

## Model Lifecycle and States

The system tracks the status of a model to manage its availability and synchronization:

- **Available**: The model exists on a remote provider (cloud) but has not been downloaded to the board yet.
- **Installed**: The model assets and the `model.yaml` descriptor are present on the board and ready for execution.
- **Detached**: An "Installed" model whose link to the original remote provider has been severed (e.g., the user logged out, or the project was deleted from the cloud). The model remains functional locally but cannot receive updates.

> **Note**: The `Available` and `Detached` states are managed by Arduino App Lab. Within `arduino-app-cli`, a model can only be `Installed` or `Broken`.

## The Model Descriptor (`model.yaml`)

The manifest file describing the Model. It is the single source of truth for the system to identify the model's capabilities and requirements.

- **Status**: Mandatory.
- **Constraint**: Must be located in the root of the model folder. Only `.yaml` extension is supported.

## model.yaml file format

The `model.yaml` file uses a structured format to define the model's identity, its execution environment, and its compatibility with various Bricks.

- **`id`** (String, Mandatory): A unique identifier for the model (e.g., a UUID or a slug).
- **`name`** (String, Mandatory): A human-readable name displayed in Arduino App Lab.
- **`runner`** (String, Mandatory): Specifies the **Runner** (always `brick`).
- **`description`** (String, Optional): A brief summary of the model's purpose and capabilities.
- **`category`** (String, Optional): The functional domain of the model (e.g., `Images`, `Audio`, `Text`).
- **`bricks`** (List of obj, Mandatory): A list of **AI Bricks** compatible with this model.
- **`metadata`** (Map, Optional): A flexible dictionary of key-value pairs for additional provider-specific information. Supports several types such as strings, integers, and booleans.

### Metadata Fields

The following metadata fields are required for Edge Impulse models:

- **`source`** (String, Mandatory): Must be set to `"edgeimpulse"`.
- **`ei-project-id`** (Integer, Mandatory): The numeric ID of the Edge Impulse project.
- **`ei-impulse-id`** (Integer, Mandatory): The numeric ID of the impulse within the project.
- **`ei-impulse-name`** (String, Mandatory): The human-readable name of the impulse.
- **`ei-model-type`** (String, Mandatory): The model type. Must be `float32`.
- **`ei-engine`** (String, Mandatory): The inference engine. Must be `tflite`.
- **`ei-deployment-version`** (Integer, Mandatory): The deployment version number from Edge Impulse.
- **`ei-last-modified`** (String, Optional): Timestamp of the last project modification in RFC3339Nano format.
- **`ei-model-url`** (String, Optional): URL to the model page on Edge Impulse Studio.
- **`ei-gpu-mode`** (Boolean, Optional): Whether the model runs in GPU mode. Defaults to `false`.


### Brick Configuration

The `bricks` list defines which AI Bricks are compatible with the model and how they are configured. The `model_configuration` for each brick must include two variables:

- **`EI_*_MODEL`**: The absolute path to the `.eim` binary. The exact variable name depends on the target brick (see the Edge Impulse Category to AI Brick Mapping section). All Edge Impulse model variables follow the naming pattern `EI_<anything>_MODEL`, where `EI_` and `_MODEL` are fixed prefix and suffix, and the middle part identifies the specific brick type (e.g., `EI_OBJ_DETECTION_MODEL`, `EI_CLASSIFICATION_MODEL`).
- **`CUSTOM_MODEL_PATH`**: The absolute path to the model's root folder (i.e., the directory containing `model.yaml`).

### Edge Impulse Category to AI Brick Mapping

The compatible bricks are determined by the **project category** set in Edge Impulse Studio. The table below lists the mapping between Edge Impulse project categories and the corresponding AI Brick IDs, along with the required `model_configuration` variable name for each brick.

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

### Complete Example - Object Detection Model

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
      EI_OBJ_DETECTION_MODEL: /home/.arduino-bricks/models/custom-ei/ei-model-842271-1/model.eim
      CUSTOM_MODEL_PATH: /home/.arduino-bricks/models/custom-ei/ei-model-842271-1
  - id: arduino:video_object_detection
    model_configuration:
      EI_V_OBJ_DETECTION_MODEL: /home/.arduino-bricks/models/custom-ei/ei-model-842271-1/model.eim
      CUSTOM_MODEL_PATH: /home/.arduino-bricks/models/custom-ei/ei-model-842271-1
metadata:
  source: edgeimpulse
  ei-project-id: 842271
  ei-impulse-id: 1
  ei-impulse-name: Hand gestures impulse
  ei-model-type: float32
  ei-engine: tflite
  ei-deployment-version: 3
  ei-model-url: https://studio.edgeimpulse.com/public/842271/live
  ei-gpu-mode: false
```

## Model Assets

Edge Impulse models use a single-file format: a self-contained `.eim` binary that bundles the model weights and the inference runtime.

## Storage Hierarchy

Models are organized on the board's storage based on their **Source** to prevent naming collisions and facilitate updates.

### **Standard Shared Path**

The default location for downloadable/custom models is typically `/home/.arduino-bricks/models/`, but it can be overridden via environment variable `ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR`.

**Note** on Directory Structure: The following tree is an example of valid configurations. The system recursively scans the root directory; the specific folder names or nesting levels do not matter, provided that each model is contained within its own folder along with its `model.yaml`.

```text
/models/
├── edge-impulse/                 # Example: Grouped by provider
│   └── impulse-123/            # A valid model folder
│       ├── model.yaml          # Mandatory metadata
│       └── model.eim           # Provider-specific asset
├── custom-projects/            # Example: Deeply nested structure
│   └── quality-control/
│       └── v1-beta/            # Also a valid model folder
│           ├── model.yaml
│           └── model_beta.eim
└── direct-model-folder/        # Example: Flat structure
    ├── model.yaml
    └── my-model.eim
```

## Resource Mapping

A Model's assets can be located anywhere within its root folder. The `model.yaml` file acts as the map, explicitly pointing the **Runner** to the correct files.

- **Local Paths**: All paths defined in the descriptor must be relative to the model's root folder.
- **Reserved Filenames**: While asset names are flexible, `model.yaml` is reserved and cannot be used for weight files.

## A complete example

A hypothetical model named "My-Face-Det":

```text
/models/my-model/
├── model.yaml              # Mandatory metadata
├── lighthouse-face.eim     # Model binary (referenced in yaml)
└── README.md               # Optional documentation for App Lab
```

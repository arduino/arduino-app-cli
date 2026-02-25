# 1. Arduino Model Specification

This is the specification for the Arduino Model format to be used with `arduino-app-cli` and `Arduino App Lab`.

Arduino Models are self-contained units that may include AI weights, configuration assets, and metadata. Within the `Arduino Uno Q board ecosystem`, models are treated as shared resources that provide the "intelligence" used by **AI Bricks**.

Unlike the application logic, models are designed to be decoupled from specific Apps, allowing multiple applications to leverage the same model concurrently while optimizing storage and memory.

### 1.1 Understanding Bricks and AI Bricks

Within the Arduino App ecosystem, a **Brick** is a modular service that acts as a standardized interface for specific functionalities.
An **AI Brick** is a specialized Brick that manages a specific AI domain or use case (e.g., _Object Detection_, _Speech-to-Text_, or _Face Detection_).
The primary goal of an AI Brick is to allow the swapping of underlying AI models without requiring changes to the application code (`main.py`), effectively hiding the technical complexities of the model's execution.

An **AI Brick** is composed of two main elements:

- **Python Module Interface**: A library that exposes unified Python methods to the App's Logic Layer. This is imported and used directly by the developer in `main.py`.
- **AI Runner**: A specialized Docker container managed by the host system. It provides the execution environment (libraries, drivers, and hardware acceleration support) where the AI model actually runs.

An AI Brick is designed to automatically determine and select the appropriate AI Runner required for a given model.

### 1.2 Model-AI Brick Relationship (N:N)

The ecosystem implements a flexible N:N (Many-to-Many) relationship between Bricks and Models:

- **Model Versatility**: A single AI Model can be compatible with multiple Bricks (e.g., a "Face Detection" model can be utilized by both an `ObjectDetection` brick and a `VideoAnalytics` brick).
- **Brick Flexibility**: A Brick can support multiple Models of the same class, allowing users to swap models (e.g., switching from a lightweight model to a more accurate one) while the Python API remains identical.
- **Concurrency**: A single AI model can be shared and utilized concurrently by multiple Bricks. In this scenario, the model assets are shared, but a separate AI Runner instance is created for each Brick. The number of active Runners will equal the number of Bricks currently accessing that model.

### 1.3 Model Types

Models within the ecosystem are categorized based on their origin and distribution method:

- **System Models**: Pre-installed models provided by Arduino. These are part of the core OS image, are read-only, and are globally available to all Apps.
- **Custom Models**: Models added to the board by the user. These can be downloaded from external providers (e.g., Edge Impulse, AI Hub) or manually imported. They are stored in a dedicated user-writable partition.

### 1.4 Custom Model Lifecycle and States

The system tracks the status of a model to manage its availability and synchronization:

- **Available**: The model exists on a remote provider (cloud) but has not been downloaded to the board yet.
- **Installed**: The model assets and the `model.yaml` descriptor are present on the board and ready for execution.
- **Detached**: An "Installed" model whose link to the original remote provider has been severed (e.g., the user logged out, or the project was deleted from the cloud). The model remains functional locally but cannot receive updates.
- **Broken(OR INVALID??)**: An installed model that fails static validation (e.g., missing weights, corrupted manifest).(TODO NOT IMPLEMENT YET: NEED A DISCUSSION)

# 2. Project Structure

An Arduino Model is defined by a directory containing its metadata and the actual model assets (e.g., weights, blobs, or configuration files). To ensure portability and allow the `arduino-app-cli` to manage models effectively, the internal structure must follow specific naming conventions and location.

While models are typically stored in a shared system directory to be used by multiple Apps, they can also be bundled within an App's folder for export to ensure the application remains self-contained when moved to a different board.(TODO NOT IMPLEMENT YET: NEED A DISCUSSION)

### 2.1 The Model Descriptor (`model.yaml`)

The manifest file describing the Model. It is the single source of truth for the system to identify the model's capabilities and requirements.

- **Status**: Mandatory.
- **Constraint**: Must be located in the root of the model folder. Only `.yaml` extension is supported.

### 2.2 Model Assets

The actual intelligence data of the model. The format and organization of these files depend on the specific **AI Runner** they are intended for.

- **Single-File Models**: Common for formats like `.eim` (Edge Impulse) or `.gguf` (Llama.cpp).
- **Directory-Based Models**: Common for complex LLMs or NPU-accelerated models (e.g., AI Hub/Genie) which require a folder containing multiple weight files and JSON configurations.

### 2.3 Storage Hierarchy

Models are organized on the board's storage based on their **Source** to prevent naming collisions and facilitate updates.

#### **Standard Shared Path**

The default location for downloadable/custom models is typically `/home/arduino/.arduino-models/` (or a similar reserved system path).

```text
/models/
├── ei/                     # Edge Impulse models
│   └── <project-id>/
│       └── <impulse-id>/
│           ├── model.yaml
│           └── model.eim
├── ai-hub/                 # Qualcomm AI Hub models
│   └── <model-uuid>/
│       ├── model.yaml
│       └── weights/
└── llamacpp/               # Llama.cpp GGUF files
    ├── model-1.yaml
    └── model-1.gguf
```

### 2.4 Resource Mapping

Unlike Apps, where the logic is always in `python/main.py`, a Model's assets can be located anywhere within its root folder. The `model.yaml` file acts as the map, explicitly pointing the **Runner** to the correct files.

- **Local Paths**: All paths defined in the descriptor must be relative to the model's root folder.
- **Reserved Filenames**: While asset names are flexible, `model.yaml` is reserved and cannot be used for weight files.

### 2.5 A complete example

A hypothetical model named "Lighthouse-Face-Det" downloaded from Edge Impulse:

```text
/models/ei/830703/4/
├── model.yaml              # Mandatory metadata
├── lighthouse-face.eim     # Model binary (referenced in yaml)
└── README.md               # Optional documentation for App Lab
```

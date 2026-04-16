# Arduino App specification

This is the specification for the Arduino App format to be used with `arduino-app-cli` and `Arduino App Lab`.

Arduino Apps are designed to run on `dual-brain` Arduino Boards (e.g., [Arduino Uno Q](https://store.arduino.cc/pages/uno-q)).
An Arduino App leverages the board's operating system and the integrated microcontroller to perform a wide range of tasks, from high-level logic, data processing, executing AI models and more.

An Arduino App is composed of two main software components that work together as a single application:

- [**Arduino Sketch**](https://docs.arduino.cc/arduino-cli/sketch-specification/). It runs on the integrated Microcontroller (MCU). It is responsible for low-level hardware interaction, sensors, and actuators.
- **Python, bricks, and containers**. They run on the board's Linux OS.

The Sketch and the Python communicate using an RPC-based messaging (see [arduino-router](https://github.com/arduino/arduino-router)).

An App can be extended through **Bricks**: modular software components that add ready-to-use functionality (e.g. databases, computer vision services). An App may include multiple Bricks. See also the [Arduino docs about bricks](https://docs.arduino.cc/software/app-lab/tutorials/bricks/).

## Arduino App folders and files

An Arduino App is a self-contained folder (called root folder) that includes a descriptor file (app.yaml), other sub-folders and files.
The root folder name must contain only alpahnumeric characters, underscore, dash and spaces.

#### app.yaml file

The `app.yaml` file contains the metadata of the App. It is mandatory and must be located in the root folder.
See the [App Descriptor (`app.yaml`)](#app-descriptor-appyaml) section for details.

#### python subfolder

Contains the python code of the App. This folder is mandatory and must be located inside the root folder of the Arduino App.
It must contain a `main.py` file (the entry point of the application) and could contain an optional `requirements.txt` file (the standard Python list of dependencies).

#### sketch subfolder

Contains the Arduino sketch to be flashed on the integrated microcontroller. This folder is optional and, if present, must include both the `sketch.ino` file (containing the main Arduino microcontroller code) and the `sketch.yaml` file (the sketch project file containing the list of Arduino libraries dependencies).\
The folder content must comply with the official [Sketch specification](https://arduino.github.io/arduino-cli/1.3/sketch-specification/).

#### README.md file

An optional markdown file, located in the root folder, used by Arduino App Lab to provide documentation to the user.
If the `description` field in `app.yaml` is absent or empty, the system derives the app description from `README.md` by parsing the first non-title paragraph and stripping all markdown syntax.

#### Reserved Folders

The following folders are reserved for specific uses:

##### `data` subfolder

A reserved directory, where the application can store persistent application data.

- The content is accessible to the user to allow backups or deletion to reset the App state.
- This folder should be emptied before sharing the App to avoid leaking personal data.
- Located in root folder of the app.

##### `.cache` subfolder

A reserved directory, containing volatile data needed to run the App (e.g., the Python `venv` folder or Docker Compose support files).

- It can be safely deleted when the App is not currently running.
- Located in root folder of the app.

#### Extra files/folders

Any other file or folder is allowed. The runtime preserves these files and makes them accessible to your Python logic, but performs no automated action on them. They are bundled with the App during Import and Export operations.

### A complete example

A hypothetical App named "SmartGarden" that adheres to the specification. Note that the root folder name (my-garden-project) differs from the App name defined in YAML.

```
my-garden-project/
├── app.yaml # Mandatory metadata (strict naming)
├── README.md # Documentation (Optional)
├── python/ # Mandatory source folder
│ ├── main.py # Mandatory entry point
│ └── requirements.txt # Python dependencies (Optional)
└── sketch/ # Arduino sketch folder (Optional)
  ├── sketch.yaml # Arduino sketch dependency
  └── sketch.ino # Arduino sketch
```

## App Descriptor (`app.yaml`)

The `app.yaml` file (also referred to as the **App Descriptor**) is the manifest of the Arduino App. It allows the host system to identify, configure, and orchestrate the App and its dependencies.

### File Format

- **Filename**: must be exactly `app.yaml`(not `.yml`).
- **Location**: Root of the App folder.
- **Syntax**: Standard YAML.

### Configuration Fields

- **`name`** (Optional): Human-readable name of the app, displayed in UI. It's recommended to keep the name similar to the root folder name of the app.
- **`icon`** (Optional): A single emoji character displayed in the UI.
- **`description`** (Optional): A short summary of the app's purpose. If omitted, it is retrieved from the first paragraph of README.md.
- **`ports`** (Optional): A list of integers representing the network ports exposed by the app.
- **`bricks`** (Optional): A list of Brick ids (e.g., `arduino:web_ui`, `arduino:image_classification`) needed by the application to perform specific tasks. Each reference may contain additional configuration options (see the next section for details).

### Brick config options

This section describes the structure of a single item within the bricks list in `app.yaml`.

- **`model`** (Optional): The AI model id to load within an AI brick.
- **`variables`** (Optional): A map of key-value pairs (both must be strings) representing configuration parameters or environment variables passed to the Brick container.
- **`devices`** (Optional): A list of strings representing the devices connected, each one corresponding (same "class") to the `required_devices` of the brick.

#### Handling Secrets in Variables

Some Bricks declare certain variables as Secrets within their own Brick definition (not in app.yaml). This is used to flag sensitive information, such as API keys or database passwords.
In app.yaml, the user sets the value of a secret variable exactly like any other variable, using the standard variables map — no special syntax is required.

Automatic Redaction: When an App is exported, the values of variables flagged as secret in the Brick's definition are automatically redacted (set to empty). This ensures that the App bundle can be safely shared without leaking personal credentials or private keys.

### Example `app.yaml`

```yaml
name: Smart Garden Pro
description: AI-powered irrigation and monitoring system
icon: 🌿
ports:
  - 5000

bricks:
  - arduino:dbstorage:
      variables:
        DB_PASSWORD: "password"
  - arduino:objectdetection:
      model: yolo-v8
      devices:
        - remote_camera_0
```

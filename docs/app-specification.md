# App Specification

This is the specification for the **Arduino App format**.

Arduino Apps are self-contained logical units designed to run on the [Arduino Uno Q](https://www.arduino.cc/product-uno-q/?utm_source=google&utm_medium=cpc&utm_campaign=US+EU-UNO-Q&gad_source=1&gad_campaignid=23081042134&gbraid=0AAAAACbEa84TLYtQ6AeV-__Mi5hhFBAsh&gclid=Cj0KCQiA4eHLBhCzARIsAJ2NZoJj4S-VRReHWeMSlKfB45wON1okCgb1bfMFtXV1LByx_SDi1977gs0aAshTEALw_wcB). They leverage the board's operating system and the integrated microcontroller to perform a wide range of tasks, from high-level logic and data processing to executing AI models and more.

Apps can be extended through Bricks. Bricks act as modular "plugins" or standardized software components—such as database connectors or specific AI model integrations that provide ready-to-use capabilities to the App.

## Layout of folders and files

An Arduino App is defined by a root folder containing its metadata, source code, and bundled resources.

**Important on Folder Naming & Location**:

- The name of the folder containing an Arduino App has no relation with the App name (defined in root-app-folder/app.yaml). It must not be assumed to be the same.
- Arduino Apps can be stored in sub-folders of a main directory (e.g., for categorization). The parsing logic is designed to handle nested structures.

While the App folder holds the application logic and local resources, the App may depend on Bricks. These dependencies are declared in the App's configuration, but the Bricks themselves are not stored inside the App folder. They are downloaded and managed externally by the host system.

### Mandatory Files

The following files and folders must be present for an App to be considered valid.

- root-app-folder/app.yaml: The metadata file describing the App.
  - Constraint: The filename must be exactly root-app-folder/app.yaml. Extensions like .yml are not supported.

- root-app-folder/python/main.py: The entry point of the application logic.
  - Constraint: The python folder must exist, and it must contain root-app-folder/main.py.

### Optional Components

**Firmware (sketch/)**
An App may include firmware to be flashed on the integrated microcontroller.

- Location: root-app-folder/sketch/ folder.
- Constraints:
  - It should contain the source code sketch.ino.
  - If the sketch.ino file exists, the app must contain a sketch.yaml configuration file ate in the same folder.
  - Compilation Scope: Only files named sketch.ino and sketch.yaml located specifically in the root-app-folder/sketch path will be processed. Firmware files located elsewhere will be ignored by the host system.

**Documentation (root-app-folder/README.md)**

- Location: Root of the App folder.
- Usage: Optional.
- Note: [Not implemented yet] The first paragraph of this file will be used as the App description in the UI, replacing the description field in app.yaml.

**Dependencies (root-app-folder/python/requirements.txt)**
If the App requires external Python libraries, they must be listed in a standard requirements.txt file located inside the python folder. The runtime environment will ensure these dependencies are installed.

**Reserved Folders**
The following folders are reserved for specific uses:

- root-app-folder/data/: Optional.
  - TODO: describe specific use case.
- root-app-folder/assets/: Optional.
  - TODO: describe specific use case.
- root-app-folder/.cache/: Optional.
  - TODO: describe specific use case.

**Extra Files**
Any other file or folder found in the main directory (or subfolders) is allowed and preserved, but otherwise ignored by the runtime system.

### A complete example

A hypothetical App named "SmartGarden" that adheres to the specification follows. Note that the root folder name (my-garden-project) differs from the App name defined in YAML.

my-garden-project/
├── app.yaml # Mandatory metadata (strict naming)
├── README.md # Documentation (Optional)
├── python/ # Mandatory source folder
│ ├── main.py # Mandatory entry point
│ ├── requirements.txt # Python dependencies(optional)
│ └── logic/ # Additional user code (extra file)
├── sketch/ # Optional firmware folder
│ ├── sketch.yaml # Mandatory if sketch.ino exists
│ └── sketch.ino # Arduino sketch
├── data/ # Reserved
└── assets/ # Reserved

## App Metadata

The core of the App definition is the metadata file called **app.yaml**.
This file allows the host system to identify and manage the App and its dependencies. It **must** be located in the root of the App folder (e.g., `root-app-folder/app.yaml`).

### app.yaml file format

The `app.yaml` file is a YAML document.

The available fields are:

- **name** - (Optional) The human-readable name of the App. It must contain only basic letters (a-z, A-Z), numbers (0-9), underscores (`_`), and dashes (`-`). It must not start with a dash.
  - _TODO: DEFINE NAME VALIDATION RULES_
- **icon** - (Optional) A single emoji character.
- **description** - (Optional) A human-readable description of what the App does.
  - _Note: This field will be deprecated and replaced by the first paragraph of the README.md file._
- **ports** - (Optional) List of integers.
  - _TODO: Describe usage._
- **required_devices** - (Optional) A list of strings representing the hardware devices used by the app.
- **bricks** - (Optional) A list of "Bricks". Additional bricks needed by the app to perform specific tasks.

### Brick Object Definition

This section describes the structure of a single item within the bricks list in app.yaml.

- id - (Mandatory) The unique identifier string of the Brick (e.g., web_ui, image_classification).
  - TODO: Update the spec after custom bricks validation rules are defined.
- model - (Optional) A string specifying a specific model or configuration profile to load within the Brick (e.g., specific AI weights or logic flavor).
- variables - (Optional) A map of key-value pairs (both must be strings) representing configuration parameters or environment variables passed to the Brick container.
- ports - (Optional) A list of integers representing the network ports that the Brick must expose to interact with the App.

**Example `app.yaml`:**

```yaml
name: Classify images
description: Image classification in the browser using a web-based interface.
ports: []
bricks:
  - arduino:web_ui: {}
  - arduino:image_classification: {}
  - arduino:mood_detector: {}
icon: 📊
```

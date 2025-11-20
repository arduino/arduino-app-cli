# Arduino App specification

This is the specification for the Arduino App format to be used with `arduino-app-cli` and `Arduino App Lab`.
An Arduino App is a combination of multiple pieces of software that interacts with each other as single application. In particular an Arduino App may be composed by:

- An Arduino Sketch that runs on a microcontroller (MCU) which performs real-time tasks interacting with sensors and actuators.
- A Python program that runs on a Linux OS which performs CPU intensive tasks (like network API, database, GUI, etc.).
- One or more AI models and runners.
- Docker containers.

The list above is not meant to be complete, and other type of components may be added in the future.
The Arduino App CLI and the Arduino AppLab Desktop IDE offers a simplified and automated way to deploy an Arduino App, taking care of all the steps needed to run the whole application (including building and uploading the firmware, or handling Docker containers).

# Arduino App Folder structure

An Arduino App is a folder including:

- The `app.yaml` file: this is the descriptor of the application in YAML format.
- A `sketch` folder: containing an [Arduino Sketch](https://arduino.github.io/arduino-cli/1.3/sketch-specification/)).
- A `python` folder: containing Python code.
- A `docs` folder reserved for the app documentation.
- A `README.md` file

The `app.yaml` file and at least one of the `sketch` or `python` folder must be present.

The Arduino App must be self-contained (it does not contain references to external files) because this means it can be exported, shared, or zipped easily.

The user-defined apps are saved into `/home/arduino/ArduinoApps` folder.
The builtin-apps are stored into `home/arduino/.local/share/arduino-app-cli/examples` folder.

Example of a `my-app` folder structure

```
my-app/
    README.md
    app.yaml
    sketch/
        sketch.ino
        sketch.yaml
    python/
        main.py
```

### `app.yaml` file descriptor

The `app.yaml` is a YAML file that describes an App. The field available in this file are:

- `name`: A short name of the app.
- `description`: A brief description of the app.
- `icon`: The emoji of the app.
- `ports`: A list of TCP ports to be exposed externally. If not given a random port is opened (if necessary).
- `bricks`: A list of bricks used by the app with its variable definitions.

Example:

```yaml
name: My Arduino App
description: An example app showcasing what you can do
icon: 🍓
ports:
  - 7000

bricks:
  - arduino/dbstorage:
      variables:
        ROOT_PASSWORD: ${secret.db_password}
        PORT: 8080
  - arduino/text-generation:
      model: gemma-1
  - arduino/objectdetection:
      model: yolo
```

### `README.md` file

This is a readme file in markdown format. Resources in the app may be linked directly, in particular images or other documentation pages in the `docs` folder.

### `sketch` folder

The `sketch` subfolder contains an Arduino Skecth, it must comply with the [Sketch specification](https://arduino.github.io/arduino-cli/1.3/sketch-specification/).

If present it must contain at least the following files:

- `sketch.ino`: The main sketch source code.
- `sketch.yaml`: The sketch project file (more info in the [Sketch Project file specification](https://arduino.github.io/arduino-cli/1.3/sketch-project-file/)).

### `python` folder

The `python` contains the Python code.

If present, it must contain the `main.py` with the python code of the main.
Optionally, a `requirements.txt` with additional python package dependencies to be installed.

### Other

Other sub-folders or files can be added to the app folder.
The reserved folder names are `sketch` and `python`.
The reserved file names are `app.yaml` and `sketch.yaml`.

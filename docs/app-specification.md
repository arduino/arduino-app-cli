# Arduino App specification
This is the specification for the `Arduino App` (from now on called App) format to be used with `arduino-app-cli` and `Arduino App Lab`.

# Arduino App Folder structure
An App is a self-contained folder that includes the following components:
 - `app.yaml` (mandatory) the file descriptor of the app in YAML format.
 - `sketch` (optional) the folder containing an Arduino [Sketch](https://arduino.github.io/arduino-cli/1.3/sketch-specification/)).
 - `python` (optional) the folder containing the Python code.

At least one on `sketch` or `python` folder must be present.
The App must be self-contained (it does not contain references to external files) because this means it can be exported, shared, or zipped easily.

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

## `README.md` file
An (optional) readme file in markdown.
The link to local resources must be in the same folder of the app. For example, a png inside the folder `myapp/docs/my-banner.png` can be referenced using ![My App](docs/my-banner.png) syntax.

### `app.yaml` file descriptor
The `app.yaml`  (or `app.yml`) is a YAML file that describes an App.

- `name`: (optional) a short name of the app.
- `description`: (optional) a brief description of the app.
- `icon`:  (optional) the emoji of the app
- `ports`:  (optional) a list of ports to be exposed externally. If not given a random port is opened (if necessary).
- `bricks` (optional) a list of bricks used by the app with its variable definitions.

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


### `sketch` sub folder
The content of the `sketch` subfolder  contains the Ardiuno skecth.
It must omply with the [Sketch specification](https://arduino.github.io/arduino-cli/1.3/sketch-specification/).

If present it must contain the followign files:
 - `sketch.ino`
 - `sketch.yaml` that is compliant to the [Sketch project file](https://arduino.github.io/arduino-cli/1.3/sketch-project-file/)

### `python` sub folder
The content of the `python` contains the python code.

If present, it must contain the `main.py` with the python code of the main.
Optionally, a `requirements.txt` with additional python package dependencies to be installed.

### Other
Other sub-folders or files can be added to the app folder.
The reserved folder names are `sketch` and `python`.
The reserved file names are `app.yaml` and `sketch.yaml`.

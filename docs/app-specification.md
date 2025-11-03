# Arduino App specification
This is the specification for the `Arduino App` (from now on called App) format to be used with `arduino-app-cli`.

An App is a self-contained folder that includes the following components:
 - `app.yaml` (mandatory) the file descriptor of the  app in YAML format.
 - `sketch` (optional) the folder containing an Arduino [Sketch](https://arduino.github.io/arduino-cli/1.3/sketch-specification/))
 - `python` (optional) the folde containin the Python code

The App must be self-contained (it does not contain references to external files) because this means it can be exported, shared, or zipped easily.

## Arduino App Folder structure
```
myapp/
    app.yaml
    sketch/
        sketch.ino
        sketch.yaml
    python/
        main.py
        requirements.txt
```


## App descriptor file
The `app.yaml` is the YAML  specificaiotn of an APP.

- `name` - the name of the app.
```yaml
name: My Arduino App
description: An example app showcasing what you can do
icon: 🍓

bricks:
  - arduino/dbstorage:
      variables:
        ROOT_PASSWORD: ${secret.db_password}
        PORT: 8080
  - arduino/text-generation:
      model: gemma-1
  - arduino/objectdetection:
      model: yolo
  - arduino/mqtt
```
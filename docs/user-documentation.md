# Arduino App CLI

## Installation requiremets

`arduino-app-cli` experience is strictly tied to Arduino hardware and software, so some limitation prevent a more general installation. In particulat the cli should only be run from an user with id `1000` that is part of `arduino`, `sysupgrade`, and `docker` group.

During the .deb installation, the package will check if an user with uid `1000` exist, if exist the required groups are set otherwise a new `arduino` user with id 1000 is created and added to the `docker`, `sysupgrade` and `arduino` group.

## Environment Variables

The following environment variables are used to configure Arduino App CLI:

| Environment Variable                   | Default Value                                    | Description                                                                        |
| -------------------------------------- | ------------------------------------------------ | ---------------------------------------------------------------------------------- |
| `ARDUINO_APP_CLI__APPS_DIR`            | `/home/arduino/ArduinoApps`                      | Path to the directory where Arduino Apps created by the user are stored            |
| `ARDUINO_APP_CLI__DATA_DIR`            | `/var/lib/arduino-app-cli`                       | Path to the directory where internal data is stored (examples, assets, properties) |
| `ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR` | `$HOME/.arduino-bricks/models`                   | Path to the directory where custom AI models are stored                            |
| `ARDUINO_APP_CLI__ALLOW_ROOT`          | `false`                                          | Allow running `arduino-app-cli` as root (**Not recommended to set to true**)       |
| `LIBRARIES_API_URL`                    | `https://api2.arduino.cc/libraries/v1/libraries` | URL of the external service used to search Arduino libraries                       |
| `DOCKER_REGISTRY_BASE`                 | `ghcr.io/arduino/`                               | Docker registry used to pull docker images                                         |
| `DOCKER_PYTHON_BASE_IMAGE`             | `app-bricks/python-apps-base:<RUNNER_VERSION>`   | Tag of the Docker image for the Python runner                                      |

## Directory Structures

Examples of user-defined Arduino Apps stored under the `ARDUINO_APP_CLI__APPS_DIR` folder.

```
├── my-first-app
│   ├── app.yaml
│   ├── README.md
│   ├── python
│   │    └── main.py
│   ├── sketch
│   │    ├── sketch.ino
│   │    └── sketch.yaml
|   └──  .cache/       # Temporary files and dependencies of the App
└── my-second-app
    ├── app.yaml
    ├── python
        └── main.py
```

Examples of the `assets` and the builtin `examples` stored under the `ARDUINO_APP_CLI__DATA_DIR` folder.

```
/var/lib/arduino-app-cli/
├── assets
│   └── 0.5.0                 # Version-specific assets
│       ├── bricks-list.yaml  # Available bricks
│       ├── models-list.yaml  # Available models
│       └── ...
├── bootloader_burned.flag
├── default.app               # Default App
├── properties.msgpack        # Variable values
├── examples                  # Built-in App examples
│   ├── air-quality-monitoring
│   │   ├── app.yaml
│   │   ├── assets
│   │   ├── python
│   │   ├── README.md
│   │   └── sketch
│   ├── anomaly-detection
│   │   ├── app.yaml
│   │   ├── assets
│   │   ├── python
│   │   └── README.md
│   └── ...
```

# Arduino App CLI

## Environment Variables

The following environment variables are used to configure Arduino App CLI:

| Environment Variable                   | Default Value                                    | Description                                                                        |
| -------------------------------------- | ------------------------------------------------ | ---------------------------------------------------------------------------------- |
| `ARDUINO_APP_CLI__APPS_DIR`            | `/home/arduino/ArduinoApps`                      | Path to the directory where Arduino Apps created by the user are stored            |
| `ARDUINO_APP_CLI__DATA_DIR`            | `/usr/share/arduino-app-cli`                     | Path to the directory where internal data is stored (examples, assets) |
| `ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR` | `$HOME/.arduino-bricks/ei-models`                | Path to the directory where custom AI models are stored                            |
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
Examples of the `assets` and the builtin `examples` are stored under the `ARDUINO_APP_CLI__DATA_DIR` folder.
```
/usr/share/arduino-app-cli/
├── assets
│   └── 0.5.0                 # Version-specific assets
│       ├── bricks-list.yaml  # Available bricks
│       ├── models-list.yaml  # Available models
│       └── ...
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

Package configurations are located in ```/home/arduino/.arduino-app-cli```
Runtime data are other files are in ```/var/lib/arduino-app-cli```
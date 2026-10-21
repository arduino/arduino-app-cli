# Arduino App CLI

## Installation requirements

The `arduino-app-cli` experience is tied to Arduino hardware and software. The CLI should run as the non-root `arduino` user, which belongs to the `arduino`, `sysupgrade`, and `docker` groups. No specific numeric UID is required.

During `.deb` installation, the package reuses the `arduino` account if it exists. Otherwise, Debian's `adduser` creates it using the first available UID allowed by the system's user-allocation policy. The account is added to the required groups, and the daemon runs as `arduino`. An unrelated account occupying UID `1000` is not used or modified.

Package configuration also ensures membership in the board's standard groups: `adm`, `video`, `users`, `sudo`, `netdev`, `dialout`, `audio`, `render`, `bluetooth`, `input`, and `gpiod`. Missing groups are created, and existing memberships are preserved. Membership in `sudo` enables administrative access according to the system's sudo policy.

App containers use the UID of the user running the CLI or daemon. Runtime directories and the optional PipeWire socket mount use `/run/user/<uid>` for that user.

Installation and upgrades set the `arduino` account's login shell to `/bin/bash`. Newly created accounts have password login disabled until a password is set.

## Environment Variables

The following environment variables are used to configure Arduino App CLI:

| Environment Variable                           | Default Value                                           | Description                                                                                                                                                                                          |
| ---------------------------------------------- | ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ARDUINO_APP_CLI__APPS_DIR`                    | `/home/arduino/ArduinoApps`                             | Path to the directory where Arduino Apps created by the user are stored                                                                                                                              |
| `ARDUINO_APP_CLI__DATA_DIR`                    | `/var/lib/arduino-app-cli`                              | Path to the directory where internal data is stored (examples, assets, properties)                                                                                                                   |
| `ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR`         | `$HOME/.arduino-bricks/models`                          | Path to the directory where custom AI models are stored                                                                                                                                              |
| `ARDUINO_APP_CLI__ALLOW_ROOT`                  | `false`                                                 | Allow running `arduino-app-cli` as root (**Not recommended to set to true**)                                                                                                                         |
| `ARDUINO_APP_CLI__REQUIRED_RUNTIMES`           | `arduino-router:arduino-router,arduino-cloud-connector` | Comma separated list of host units whose socket is bind-mounted into the App containers, each in the form `<unit>[:<group>]` where the optional group is added to the container to access the socket |
| `ARDUINO_APP_CLI__PLATFORM_VERSION_CONSTRAINT` | `<2.0.0-0`                                              | Semver constraint that limits the version of the Arduino platform installed and offered as an update                                                                                                 |
| `LIBRARIES_API_URL`                            | `https://api2.arduino.cc/libraries/v1/libraries`        | URL of the external service used to search Arduino libraries                                                                                                                                         |
| `DOCKER_REGISTRY_BASE`                         | `ghcr.io/arduino/`                                      | Docker registry used to pull docker images                                                                                                                                                           |
| `DOCKER_PYTHON_BASE_IMAGE`                     | `app-bricks/python-apps-base:<RUNNER_VERSION>`          | Tag of the Docker image for the Python runner                                                                                                                                                        |

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
├── pipewire_linger.timestamp # PipeWire linger record
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

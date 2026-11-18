# Arduino App CLI

## Installation requiremets

`arduino-app-cli` experience is strictly tied to Arduino hardware and software, so some limitation prevent a more general installation. In particulat the cli should only be run from an user with id `1000` that is part of `arduino`, `sysupgrade`, and `docker` group.

During the .deb installation, the package will check if an user with uid `1000` exist, if exist the required groups are set otherwise a new `arduino` user with id 1000 is created and added to the `docker`, `sysupgrade` and `arduino` group.

## Container image references

Image availability is checked against the board's Docker engine using Docker's
reference resolution. An image without a tag, such as `viktoruj/ping_pong`, refers
to `viktoruj/ping_pong:latest`. If that image is already local, it satisfies the
app's readiness check and is not downloaded again during preparation.
Use an explicit version tag or digest when you need a specific image version.

## Bricks that build their container

A brick compose service with `build:` is built at every app start, and a relative
`context` is the brick folder. The build uses the legacy builder of the Docker engine,
not BuildKit: BuildKit-only Dockerfile syntax, such as `RUN --mount` or heredocs, fails.
A release build refuses an app with such a brick. A start with `--no-prepare` does not
build: it uses the image of the last build, and fails if there is none.

## Third-party Bricks in Apps

An App can use third-party Bricks from the bricks index, as described in the [App specification](app-specification.md#third-party-bricks). Add them when creating the App with `arduino-app-cli app new my-app --bricks acme:weather` or, to pin a version, `--bricks acme:weather@1.2.0`.

- `app start` downloads the bricks index when it is missing from the board or older than one hour, and installs the Bricks that are not installed yet. If an old index cannot be updated, the saved one is used, so the App still starts offline.
- `app start --no-prepare` never accesses the network: it fails if the bricks index or a Brick is missing.
- The Python code of the App imports a third-party Brick by its name, e.g. `import weather`: `app start` mounts the Python package of each Brick in the App container.
- `brick uninstall` refuses to remove a Brick version that a running App uses: stop the App first.
- Releases do not ship third-party Bricks: building a release installs the Bricks the App uses on the board running the build, and pins each unpinned one, in the `app.yaml` of the release, to the version the App uses there. That version must also support the target board, otherwise the build fails: pin a version that supports both. Preparing the release on the board (`app install`, `app start`, or the prepare endpoint of the API) installs the pinned versions, and a release starts only when they are installed and still listed in the saved bricks index.

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
├── brick-index-contrib.json  # Bricks index, from `brick update-index`
├── bricks
│   └── <namespace>
│       └── <name>
│           └── <version>     # Brick installed with `brick install`, removed with `brick uninstall`
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

# Arduino App CLI

`arduino-app-cli` is a command line tool and a service running on Arduino UNO Q boards, that:

- manages and runs Arduino Apps on the board (both Linux and microcontroller parts)
- provides multiple APIs to perform actions and fetch data, used by the front-end (ArduinoAppsLab)
- auto-updates itself and other components

## Environment Variables

The following environment variables are used to configure `arduino-app-cli`:

### Application Directories

- **`ARDUINO_APP_CLI__APPS_DIR`** Path to the directory where Arduino Apps created by the user are stored.\
  **Default:** `/home/arduino/ArduinoApps`

- **`ARDUINO_APP_CLI__DATA_DIR`** Path to the directory where internal data is stored.\
  **Default:** `/home/arduino/.local/share/arduino-app-cli`\
  This folder contains:
  - **`examples/`** default example Apps (_e.g._ `/home/arduino/.local/share/arduino-app-cli/examples`)
  - **`assets/`** contains a subfolder for each asset version (_e.g._ `/home/arduino/.local/share/arduino-app-cli/assets/0.4.5`)
    - Each asset folder includes:
      - `bricks-list.yaml`
      - `models-list.yaml`
  - **other data** such as `properties.msgpack` containing variable values

- **`ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR`** Path to the directory where custom models are stored.\
  **Default:** `$HOME/.arduino-bricks/ei-models`\
  (_e.g._ `/home/arduino/.arduino-bricks/ei-models`)

---

### Execution Settings

- **`ARDUINO_APP_CLI__ALLOW_ROOT`** Allow running `arduino-app-cli` as root.\
  **Default:** `false` **Not recommended to set to true.**

---

### External Services

- **`LIBRARIES_API_URL`** URL of the external service used to search libraries.\
  **Default:** `https://api2.arduino.cc/libraries/v1/libraries`

---

### Docker Settings

- **`DOCKER_REGISTRY_BASE`** Docker registry used to pull images.\
  **Default:** `ghcr.io/arduino/`

- **`DOCKER_PYTHON_BASE_IMAGE`** Tag of the Docker image for the Python runner.\
  **Default:** `app-bricks/python-apps-base:<RUNNER_VERSION>`

### App folder and persistent data

When running an app, persistent files will be saved in the `data` folder inside the app folder; other supporting files, including the Python venv are saved in the `.cache` folder inside the app folder.

### Docker images registry

Arduino Apps bricks might required a docker image, in that case the orchestrator will pull those from the registry configured with the `DOCKER_REGISTRY_BASE` environment variable. By default this points to an Arduino GitHub Container Registry (ghcr.io/arduino).

The only image that needs to be referenced directly is the base Python image (`DOCKER_PYTHON_BASE_IMAGE`), all other containers can be downloaded automatically by the orchestrator depending on the bricks specified as dependencies in the app.yml file.

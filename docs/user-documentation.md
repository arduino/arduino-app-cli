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

## Arduino App Releases

An **Arduino App Release** is a self-contained, reproducible `.tar.gz` archive of an app that has already been built, designed to be moved to another board and launched without recompiling the sketch or downloading Python dependencies. Only the required containers are pulled at start.

This is different from `app export`/`app import`, which bundle only the *source* of an app (and deliberately exclude the `.cache` folder). A release instead **also includes the build artifacts** — the compiled sketch binary, the provisioned Python virtual environment and a version-pinned compose file — plus a manifest and any required AI models.

Each release carries a **release number** (set with `--release-number`, or a `YYYYMMDDhhmmss` timestamp by default). Once **installed**, a release becomes a *regular app*: it is flagged as a release in its `app.yaml` (with the release number) and launched with the ordinary `app start` command — there is no separate "run release" command. Because it is flagged, the run command flashes the prebuilt sketch and uses the frozen compose instead of recompiling or re-provisioning.

### What's inside a release

A release is a gzip-compressed tar archive containing a single root folder named after the app (lowercased, spaces replaced with `-`). Its layout:

```
my-app.tar.gz
└── my-app/                           # release root (sanitized app name)
    ├── arduino-app-release.yaml      # release manifest (provenance, compatibility, models)
    ├── app.yaml                      # app descriptor (secret values scrubbed by default)
    ├── README.md                     # optional, if present in the app
    ├── python/                       # user Python code
    │   ├── main.py
    │   └── requirements.txt          # optional
    ├── sketch/                       # user sketch sources (if the app has a sketch)
    │   ├── sketch.ino
    │   └── sketch.yaml
    ├── bricks/                       # optional app-local (custom) bricks
    ├── .cache/                       # build artifacts (normally volatile, shipped here)
    │   ├── release-compose.yaml      # flattened, version-pinned, relocatable compose
    │   ├── sketch/                   # prebuilt sketch binary (flashed as-is, no recompile)
    │   └── <venv>/                   # provisioned Python virtualenv (pip is not re-run)
    └── models/                       # bundled AI model artifacts (only if the app needs them)
        └── <repository>/<files>      # mirrors the layout under the custom-models dir
```

What each part is for:

- **`app.yaml`, `README.md`, `python/`, `sketch/`, `bricks/`** — the app as authored, except that secret brick variables in `app.yaml` are scrubbed to `${NAME}` placeholders by default (see [Secrets](#secrets)).
- **`.cache/sketch/`** — the already-compiled microcontroller binary. On start it is flashed directly; the sketch is never recompiled.
- **`.cache/<venv>/`** — the provisioned Python virtual environment. Because it runs inside the container at a fixed mount point, it is portable; Python dependencies are not re-installed on the destination.
- **`.cache/release-compose.yaml`** — a single self-contained compose file produced by flattening the app's generated compose and pinning concrete container image versions. Host-specific values are tokenized so the file is relocatable: `${APP_HOME}` (app folder), `${HOST_IP}`, and `${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}` (models dir). On install it is **localized** to the destination and written as the standard `.cache/app-compose.yaml` (the app path and models dir are resolved; `${HOST_IP}` is left for the run command to fill, exactly as for a normal app).
- **`models/`** — bundled AI model files, present only when the app references non-preloaded models (see [AI models](#ai-models)).
- **`arduino-app-release.yaml`** — the manifest (see below).

#### The manifest (`arduino-app-release.yaml`)

The manifest records provenance and compatibility, and is what marks an installed app as release-managed. Example:

```yaml
format_version: "1.0"
app_name: Smart Garden Pro
release: "20260627100000"        # --release-number, or a timestamp by default
created_at: 2026-06-27T10:00:00Z
source_cli_version: 1.2.3
runner_version: 0.11.0rc6
python_image: ghcr.io/arduino/app-bricks/python-apps-base:0.11.0rc6
board_name: unoq                 # board the release was built for
fqbn: arduino:zephyr:unoq
arch: arm64
images:                          # all container images referenced by the frozen compose
  - ghcr.io/arduino/app-bricks/python-apps-base:0.11.0rc6
  - ghcr.io/arduino/app-bricks/object-detection:1.4.0
models:                          # AI models required by the app and how they are provided
  - id: "ei:efficientnet-b4"
    name: General purpose object classification - EfficientNet-B4
    runner: brick
    preloaded: false             # false => its files are bundled below
    bundled: true
    paths:                       # artifact paths, relative to the custom-models dir
      - edge-impulse/efficientnet-b4-qnn.eim
required_secrets:                # secret variables to provide on the destination (values NOT in release)
  - name: DB_PASSWORD
    brick: arduino:dbstorage
```

Field reference:

| Field | Meaning | Used at runtime? |
| ----- | ------- | ---------------- |
| `format_version` | Version of the release format itself. | informational |
| `app_name` | Human-readable app name. | cosmetic (shown by `install`) |
| `release` | The release release number (from `--release-number`, or a `YYYYMMDDhhmmss` timestamp). Stamped into the installed app's `app.yaml`. | ✅ |
| `created_at` | Build timestamp (UTC). | informational |
| `source_cli_version` | Version of `arduino-app-cli` that built the release. | informational |
| `runner_version` | The app-bricks **runner generation** that produced the bricks/models and the Python base image (e.g. `0.11.0rc6`). | informational |
| `python_image` | The base image of the **main Python container** — the one the bundled venv runs inside and is ABI-tied to. | informational |
| `board_name` / `fqbn` / `arch` | The board the release was built for. | ✅ compatibility check |
| `images` | The **complete** list of every container image referenced by the frozen compose (the Python base image **plus** all brick and service images). | informational |
| `models` | The AI models the app requires and how each is provided (`preloaded`, `bundled`, and bundled `paths`). | ✅ model restore |
| `required_secrets` | Secret variables (name + owning brick) whose values are **not** in the release and must be supplied on the destination. | ✅ secrets template + run-time check |

**On the apparent overlap between `images`, `python_image` and `runner_version`:** the **frozen compose is the single source of truth** for which containers are pulled and run — at start the runtime only reads the compose (`docker compose up --pull missing`). The manifest's `images`, `python_image` and `runner_version` are **provenance metadata and are not read at runtime**; they exist so a person or tool can answer questions without parsing the compose:

- `images` — *"everything this release needs to pull"* (an unlabeled flat list).
- `python_image` — *"which single image is THE Python runtime"*. It also appears inside `images`, but is called out separately because the bundled venv only works with that exact image, so it is the most important one to identify. 
- `runner_version` — *"which app-bricks generation built this"*. It usually matches the tag in `python_image`, but can differ when a custom Python base image is used (`DOCKER_PYTHON_BASE_IMAGE`), which is why both are recorded.

Only `release`, `board_name`/`fqbn`/`arch` and `models` affect what `install`/`start` actually do; the rest is for inspection and auditing.

On install, `board_name`/`fqbn`/`arch` are checked against the destination board (override with `--force`).

#### What's excluded

- The **`data/`** folder (user/runtime data — may contain personal data).
- The host-specific generated compose files **`.cache/app-compose.yaml`** and **`.cache/app-compose-overrides.yaml`** (they bake absolute host paths and are superseded by `release-compose.yaml`).

> **Security note:** by default a release contains **no secrets** — secret brick variables are scrubbed (see [Secrets](#secrets)). If you build with `--keep-secrets`, the values are embedded in `app.yaml` and the frozen compose; treat such an archive as sensitive.

### Where things land on install — and how it runs afterwards

`release install` extracts the release into the apps directory and restores bundled models into the custom-models dir. During install it also **localizes** the app: it produces `.cache/app-compose.yaml` from the frozen compose and stamps the release metadata into `app.yaml`.

```
$ARDUINO_APP_CLI__APPS_DIR/
└── my-app/
    ├── app.yaml                      # now carries: frozen_release: { number: "<number>" }
    ├── arduino-app-release.yaml      # the manifest, preserved next to app.yaml
    ├── python/  sketch/  ...
    └── .cache/
        ├── app-compose.yaml          # localized, version-pinned compose (HOST_IP left dynamic)
        ├── sketch/                   # prebuilt binary
        └── <venv>/

$ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR/
└── <repository>/<files>             # restored from the release's models/ folder
```

From here the app is **a regular app**: it shows up in `app list` and is launched, stopped and inspected with the ordinary commands:

```sh
arduino-app-cli app start my-app     # flashes the prebuilt binary, brings up the frozen compose
arduino-app-cli app stop my-app
```

Because `app.yaml` is flagged as a release, `app start` takes the release path instead of the normal build path:

- **Sketch:** the prebuilt binary from `.cache/sketch/` is flashed to the microcontroller as-is (an upload-only step) — it is **not** recompiled.
- **Python:** the on-disk venv and the localized `.cache/app-compose.yaml` are used directly — the environment is **not** re-provisioned and dependencies are **not** re-installed.
- **Containers:** only missing images are pulled (`--pull missing`).

Everything else (`app stop`, `app logs`, status) behaves exactly as for a normal app. (To get a normal, editable app back, use `release clone`, described below.)

### AI models

Required AI models are made part of the release at **create** time, so that installing it never downloads a model — the only thing ever pulled from the network is containers (at `start`).

- **Preloaded models** (baked into a brick/base image) ride along with the version-pinned container — nothing extra to bundle.
- **Every other required model** (custom / Edge Impulse models and handler-downloaded models such as those from AI Hub, Edge Impulse or Hugging Face) is **bundled into the `.tar.gz`** from its on-disk files under the custom-models dir (`ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR`). `install` restores them into the destination's custom-models dir — it only copies from the archive, it never downloads.

`release build` fails if a required, non-preloaded model is not installed / its files cannot be located (the release would otherwise be incomplete) — install it first, or pass `--no-models` to deliberately exclude models and install them on the destination yourself.

On install, a bundled model artifact that already exists on the destination is kept as-is (use `--force` to overwrite). The frozen compose references the models dir via a `${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}` placeholder, so it resolves correctly wherever the destination keeps its models.

### Secrets

A release, unlike a model, must **never** carry credentials. Brick variables that the brick definition flags as `secret: true` (API keys, passwords, …) are handled specially so the shareable archive contains no secret material.

**At create (default):** every secret variable that has a value is **scrubbed** — its value is replaced with a `${NAME}` env-variable placeholder in both `app.yaml` and the frozen compose, and the secret is recorded (name + owning brick) under `required_secrets` in the manifest. The value itself is **not** written anywhere in the release. (Empty secret variables are left as-is — there is nothing to hide.)

**At install:** a template is written to the app's **`data/secrets.env`** (only if it doesn't already exist), listing the required keys with empty values, e.g.:

```
# Secrets required by this Arduino App Release.
# Fill in the values below, then start the app.
# required by brick arduino:dbstorage
DB_PASSWORD=
```

`data/` is the reserved per-app data folder — it is excluded from releases and exports, so the values you put there stay on the machine and are never re-released.

**At run (`app start`):** the values in `data/secrets.env` are loaded and injected into the environment so the `${NAME}` placeholders in the compose resolve. If any required secret is missing or empty, `app start` **fails fast** with a clear message naming the missing keys — the app does not start with blank credentials.

**Escape hatch:** `release build --keep-secrets` embeds the secret values in the release instead of scrubbing them (no `${NAME}` placeholders, no `required_secrets`). The resulting archive is self-contained but **sensitive** — only use it for trusted, private transfers.

> The secret values still live in plaintext in `data/secrets.env` on the destination (that is unavoidable without an external secret manager); the guarantee is only that the **release** carries no secrets.

**Same mechanism for `app export`:** `app export --scrub-secrets` applies the identical placeholder scrubbing to a plain source export (instead of the default behavior, which empties secret values). The exported zip then carries a `data/secrets.env` template, and the re-imported app — once its secrets are filled in — resolves them at `app start` exactly as a release does. (Without `--scrub-secrets`, `app export` keeps emptying secret values as before.)

### Commands

| Command | Description |
| ------- | ----------- |
| `arduino-app-cli release build <app_path> <output.tar.gz>` | Build a release from a **pre-built** app. The app must have been started at least once (the command never compiles the sketch nor provisions the venv). Required on-disk AI models are bundled; secrets are scrubbed. Flags: `--release-number <n>` (default: `YYYYMMDDhhmmss`), `--overwrite`, `--no-models`, `--keep-secrets`. |
| `arduino-app-cli release install <release.tar.gz>` | Install a release into the apps directory; bundled models are restored into the custom-models dir and the app is flagged as a release in `app.yaml`. By default it then **prepares** the app (pre-pulls its container images) so the first `app start` needs no network. Afterwards run it with `app start`. Use `-` to read from stdin, `--force` to install a release built for a different board / overwrite existing models, and `--no-prepare` to skip the image pre-pull (run `release prepare` later). |
| `arduino-app-cli release prepare <release_app>` | Pre-pull the container images the installed app needs (per its compose file) so a later `app start` finds them locally. The app is **not** started and no AI models are downloaded (only containers). This is the same step `install` runs by default; use it after `--no-prepare`, or to re-pull. |
| `arduino-app-cli release clone <release_app> <new_app_name>` | Create a new **editable** app from an installed release: removes the prebuilt artifacts, the manifest and the release flag, so the next `app start` recompiles and re-provisions normally. |

> There is no `release start` command. An installed release is launched with the regular `app start` (and stopped with `app stop`).

### Typical workflow

```sh
# On the source board (app already started at least once):
arduino-app-cli app start my-app
arduino-app-cli release build my-app my-app.tar.gz --release-number 1.2.0

# On the destination board:
arduino-app-cli release install my-app.tar.gz   # extracts + pre-pulls container images
arduino-app-cli app start my-app            # runs as a release: no recompile, no pip

# ...or split the image pull from the install:
arduino-app-cli release install my-app.tar.gz --no-prepare
arduino-app-cli release prepare my-app          # pre-pull images later (no start)

# To turn it back into an editable app:
arduino-app-cli release clone my-app my-app-dev
arduino-app-cli app start my-app-dev        # recompiles + re-provisions as usual
```

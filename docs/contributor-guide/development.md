<!-- Source: https://github.com/arduino/tooling-project-assets/blob/main/documentation-templates/contributor-guide/other/development.md -->

# Development Guide

> [!NOTE]
> The `arduino-app-cli` is designed to run on the Board and access peripherals that are not available on a development PC (e.g., the microcontroller).
>
> For easier testing, using an **Arduino UNO Q** is recommended, as local testing is limited to functionalities that do not require board-specific features.

## Prerequisites

The following development tools must be available in your local environment:

- [Go](https://go.dev/dl/)
- [Task](https://taskfile.dev/)
- [adb client](https://developer.android.com/tools/adb) [optionally]

## Building the Project

- `task init`
- `task build`
- `task generate:assets` to download locally the assets of the [Arduino Bricks](`https://github.com/arduino/app-bricks-py`)
- `ARDUINO_APP_CLI__DATA_DIR=debian/arduino-app-cli/home/arduino/.local/share/arduino-app-cli task start` to build and start the arduino-app-cli in daemon mode.

## Running Checks

Checks and tests are set up to ensure the project content is functional and compliant with the established standards.

- `task fmt-check`
- `task lint`
- `task test`

## Installing arduino-app-cli into the board

This is reccomended way to test a local development version of the arduino-app-cli into a board.

1. Connect an [Arduino UNO Q](https://docs.arduino.cc/hardware/uno-q/) board via USB.
1. `task board:install` installs the current version of Arduino App CLI on the board (`adb` is needed). The password of the `arduino` username of the board is requested.

## Automatic Corrections

Tools are provided to automatically bring the project into compliance with some of the required checks.

- `task lint`
- `task fmt`

## Generate API docs

If a PR, change the HTTP API definitions, the following steps are needed:

1. Open the `cmd/gendoc/docs.go` and modify/add/remove the definitions
1. Run `task doc` to generate the docs (i.e., the files `internal/api/docs/openapi.yaml` and `internal/e2e/client/client.gen.go` are generated)

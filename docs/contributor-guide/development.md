<!-- Source: https://github.com/arduino/tooling-project-assets/blob/main/documentation-templates/contributor-guide/other/development.md -->

# Development Guide

> [!NOTE]
> The `arduino-app-cli` is designed to run on the Board and access peripherals that are not available on a development PC.
>
> For easier testing, using an **Arduino UNO Q** is recommended, as local testing is limited to functionalities that do not require board-specific features.

## Prerequisites

The following development tools must be available in your local environment:

- [Go](https://go.dev/dl/)
- [Docker](https://docs.docker.com/engine/install/)
- [adb client](https://developer.android.com/tools/adb) [optionally]

## Building the Project

---
❗ Building on Windows machines is not supported.
---

Build the project (run once):

- `go tool task init`
- `go tool task build`
- `go tool task generate:assets` to download locally the assets of the [Arduino Bricks](https://github.com/arduino/app-bricks-py)

Start the arduino-app-cli in daemon mode:

- `ARDUINO_APP_CLI__DATA_DIR=debian/arduino-app-cli/var/lib/arduino-app-cli go tool task start`

NOTE: only a subset of HTTP APIs are working by running the daemon mode on a development PC. To run Arduino App CLI on the board see the **Running Arduino App CLI on the board** section below.

## Running Checks

> [!NOTE]
> Since Arduino App CLI runs on a Debian-based OS, some tests do not work on Windows and macOS

Checks and tests are set up to ensure the project content is functional and compliant with the established standards.

- `go tool task fmt-check`
- `go tool task lint`
- `go tool task test`

In particular, `go tool task test` runs the following tests

- `test:pkg` which exposes a cross-platform API for working with the board (those should run for every platform)
- `test:internal` runs tests of the internal components, which targets only Linux

## Running Arduino App CLI on the board

This is reccomended way to test a local development version of Arduino App CLI on a board.

1. Connect an [Arduino UNO Q](https://docs.arduino.cc/hardware/uno-q/) board via USB.
1. `go tool task board:install` installs the current version of Arduino App CLI on the board (`adb` is needed). The password of the `arduino` username of the board is requested.

## Automatic Corrections

Tools are provided to automatically bring the project into compliance with some of the required checks.

- `go tool task fmt`

## Generate API docs

If a PR, change the HTTP API definitions, the following steps are needed:

1. Open the `cmd/gendoc/docs.go` and modify/add/remove the definitions
1. Run `go tool task doc` to generate the docs (i.e., the files `internal/api/docs/openapi.yaml` and `internal/e2e/client/client.gen.go` are generated)

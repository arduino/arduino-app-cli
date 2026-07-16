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

Set up the local environment (run once):

- `go tool task init`

To run Arduino App CLI on the board see the **Running Arduino App CLI on the board** section below.

## Updating the Python runner version

The python runner assets are no longer tracked in the repo; they're downloaded on demand at `.deb` build time and when running e2e tests.

1. Bump `RUNNER_VERSION` in `Taskfile.yml`.
2. Run `go tool task bump:runner-version`. It updates `RunnerVersion` in `internal/orchestrator/config/config.go` to match.
3. Commit the changes to `Taskfile.yml` and `internal/orchestrator/config/config.go`.

`build-deb` and `test:internal` download the runner assets on their own (via `download-runner-assets`) and cache them by version.

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

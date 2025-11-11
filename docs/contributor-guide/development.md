<!-- Source: https://github.com/arduino/tooling-project-assets/blob/main/documentation-templates/contributor-guide/other/development.md -->

# Development Guide

## Prerequisites

The following development tools must be available in your local environment:

- [Go](https://go.dev/dl/)
- [Task](https://taskfile.dev/)
- [ddb client](https://developer.android.com/tools/adb) [optionally]

## Building the Project

- `task init`

## Uploading the arduino-app-cli into the board
Connect an [Arduino UNO Q](https://www.arduino.cc/product-uno-q) board to the PC via USB.

 - `task board:install` it installs the current `arduino-app-cli` inside the board (`adb` is needed). The password of the `ardiuno` username of the board is requested.

## Running Checks

Checks and tests are set up to ensure the project content is functional and compliant with the established standards.

- `task fmt-check`
- `task test`

## Automatic Corrections

Tools are provided to automatically bring the project into compliance with some of the required checks.

- `task lint`
- `task fmt`

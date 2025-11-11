<!-- Source: https://github.com/arduino/tooling-project-assets/blob/main/documentation-templates/contributor-guide/other/development.md -->

# Development Guide

## Prerequisites

The following development tools must be available in your local environment:

- [Go](https://go.dev/dl/)
- [Task](https://taskfile.dev/)
- [adb client](https://developer.android.com/tools/adb) [optionally]

## Building the Project

- `task init`

## Running Checks

Checks and tests are set up to ensure the project content is functional and compliant with the established standards.

- `task fmt-check`
- `task test`

## Testing arduino-app-cli into the board
Connect an [Arduino UNO Q](https://docs.arduino.cc/hardware/uno-q/) board via USB.

 - `task board:install` installs the current version of Arduino App CLI on the board (`adb` is needed). The password of the `arduino` username of the board is requested.


## Automatic Corrections

Tools are provided to automatically bring the project into compliance with some of the required checks.

- `task lint`
- `task fmt`


## Generate API docs
If th
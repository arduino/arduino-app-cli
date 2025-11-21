# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)

## [Unreleased]

### Added

- Improve the help message of `arduino-app-cli` (https://github.com/arduino/arduino-app-cli/pull/61)[#61]
- Add `arduino-avahi-serial.service` systemd service that runs once to add the `serial_number` to the avahi daemon (https://github.com/arduino/arduino-app-cli/pull/48)[#48]
- Add the `daemon_version` field to the `arduino-app-cli version` command (https://github.com/arduino/arduino-app-cli/pull/49)[#49]
- Add `arduino-app-cli app clean-cache <app-id>` command (https://github.com/arduino/arduino-app-cli/pull/59)[#59]
- pkg/board/board: expose the `serial_number` in network mode (https://github.com/arduino/arduino-app-cli/pull/63)[#63]
- pkg/board/remote/adb: return `failed to create command` error when `GetCmd` fails (https://github.com/arduino/arduino-app-cli/pull/79)[#79]
- Update `arduino-cli` dependencies (https://github.com/arduino/arduino-app-cli/pull/86)[#86]
- Add `code` field with `NO_INTERNET_CONNECTION`, `OPERATION_IN_PROGRESS` and `UNKNOWN_ERROR` values in the update handlers and CLI (https://github.com/arduino/arduino-app-cli/pull/62)[#62]
- Refactor README.md and contribution guidelines (https://github.com/arduino/arduino-app-cli/pull/58)[#58]
- Return detailed `error running needrestart command` error information during updates (https://github.com/arduino/arduino-app-cli/pull/95)[#95]

### Removed

- Remove `arduino-app-cli app ps` command (https://github.com/arduino/arduino-app-cli/pull/65)[#65]

### Fixed

- pkg/board/remote/adb: fix zombie process in adb that resulted in `fork failed: resource temporarily unavailable` error (https://github.com/arduino/arduino-app-cli/pull/81)[#81]
- Check if the app is running/starting before stopping it in `arduino-app-cli app stop` and `DELETE /v1/apps/{appID}` (https://github.com/arduino/arduino-app-cli/pull/84)[#84]
- Allow missing required variables when adding a brick (https://github.com/arduino/arduino-app-cli/pull/74)[#74]

## [0.6.7] - 2025-11-10

### Added

- Flash sketch in ram (https://github.com/arduino/arduino-app-cli/pull/12)[#12]
- Return `config_variables` field in `/apps/:id/bricks` and `apps/:id/bricks/:id` (https://github.com/arduino/arduino-app-cli/pull/18)[#18]
- Add brick details completions (https://github.com/arduino/arduino-app-cli/pull/54)[#54]

### Fixed

- Install libraries missing from local library-index (https://github.com/arduino/arduino-app-cli/pull/50)[#50]
- Remove the `variable default value cannot be empty` error when adding a brick to an app (https://github.com/arduino/arduino-app-cli/pull/44)[#44]

## [0.6.6] - 2025-11-03

### Added

- Improve `arduino-app-cli restart` command (https://github.com/arduino/arduino-app-cli/pull/37)[#37]
- Return adb stdout/err in case of error (https://github.com/arduino/arduino-app-cli/pull/40)[#40]
- Add `used_by_apps` field to the `GET /v1/bricks/{brickID}` (https://github.com/arduino/arduino-app-cli/pull/30)[#30]

### Fixed

- Fix websocket origin validation (fixes serial monitor on Windows) (https://github.com/arduino/arduino-app-cli/pull/39)[#39]
- Use a valid origin in `GET /v1/monitor/ws` (https://github.com/arduino/arduino-app-cli/pull/41)[#41]
- Remove `Requires` from the systemd `arduino-app-cli.service` (https://github.com/arduino/arduino-app-cli/pull/34)[#34]

## [0.6.5] - 2025-10-27

### Removed

- Remove the `arduino-app-cli board` sub-command (https://github.com/arduino/arduino-app-cli/pull/27)[#27]
- Remove the internal zephyr core (https://github.com/arduino/arduino-app-cli/pull/28)[#28]

## [0.6.3] - 2025-10-27 [YANKED]

The zephyr core index contains errors, causing updates to the next version to fail.

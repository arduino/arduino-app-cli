# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)

## [Unreleased]

### Added
- Improve the help message of `arduino-app-cli` (#61)
- Add `arduino-avahi-serial.service` systemd service which runs once adding the `serial_number` to the avahi daemon (#48)
- Add the `daemon_version` in the  `arduino-app-cli version` command (#49)
- Add  `arduino-app-cli app clean-cache <app-id>` command (#59)
- pkg/board/board: expose the `serial_number` in network mode (#63)
- pkg/board/remote/adb: return `failed to create command` if the `GetCmd` fails (#79)
- Update `arduino-cli` dependencies (#86)
- Add `code` field with `NO_INTERNET_CONNECTION`,  `OPERATION_IN_PROGRESS` and `UNKNOWN_ERROR` values in the update handlers and CLI (#62)
- Refactor README.md + contribution guidelines (#58)
- Return the `error running needrestart command` error detail during the update (#95)

### Removed
- Remove `arduino-app-cli app ps` command (#65)

### Fixed
- pkg/board/remote/adb: fix zombie process in adb resulting in  `fork failed: resource temporarily unavailable` error (#81)
- Check if the app is running/starting before stopping it in `arduino-app-cli app stop` and `DELETE /v1/apps/{appID}` (#84)
- Allow missing required variable when a brick is added (#74)

## [0.6.7] - 2025-11-10

### Added

- Flash sketch in ram (#12)
- Return `config_variables` field in `/apps/:id/bricks` and `apps/:id/bricks/:id` (#18)
- Add brick details completions (#54)

### Fixed

- Install a library missing from local library-index (#50)
- Remove the `variable default value cannot be empty` error when a brick is added to an app (#44)

## [0.6.6] - 2025-11-03

### Added

- Improve `arduino-app-cli restart` command (#37)
- Return adb stdout/err in case of error (#40)
- Add `used_by_apps` field to the `GET /v1/bricks/{brickID}` (#30)

### Fixed

- Check websocket origin check (fixes serial monitor on Windows) (#39)
- Use a valid origin in `GET /v1/monitor/ws` (#41)
- Remove `Requires` from the systemd `arduino-app-cli.service` (#34)

## [0.6.5] - 2025-10-27

### Removed

- Remove the `arduino-app-cli board` sub-command (#27)
- Remove the internal zephyr core (#28)

## [0.6.3] - 2025-10-27 [YANKED]

The index of the zephyr core is wrong, so the update to the next version fails.

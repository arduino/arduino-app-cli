# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)

## [Unreleased]

### Added

- Add the `require_model` to the `GET /v1/bricks` and `GET /v1/bricks/id` [#93](https://github.com/arduino/arduino-app-cli/pull/93)
- Add the `require_model` to the `GET /v1/apps/{id}/bricks` and `GET /v1/apps/{id}/bricks/{id}` [#93](https://github.com/arduino/arduino-app-cli/pull/93)
- Add the `compatible_models` to the `GET /v1/bricks/{id}` [#94](https://github.com/arduino/arduino-app-cli/pull/94)
- Add the `compatible_models` to the `GET /v1/apps/{id}/bricks/{id}` [#99](https://github.com/arduino/arduino-app-cli/pull/99)
- Bump [examples](https://github.com/arduino/app-bricks-examples) to 0.5.1 [#100](https://github.com/arduino/arduino-app-cli/pull/100)
- mount `/dev/snd/by-id` in python container [#119](https://github.com/arduino/arduino-app-cli/pull/119)


### Changed
- Do not exit from default Python script  [#106](https://github.com/arduino/arduino-app-cli/pull/106)

### Deprecated

### Removed

- Remove the `models` fields from the `GET /v1/bricks` response [#94](https://github.com/arduino/arduino-app-cli/pull/94)
- Remove deprecated `require_devices` field from the `brick-list.yaml` [#111](https://github.com/arduino/arduino-app-cli/pull/111)
- Remove the `compatible_models` field from the `GET /v1/apps/{id}/bricks` response [#114](https://github.com/arduino/arduino-app-cli/pull/114)
- Remove `--no-python` from the CLI `arduino-app-cli app new name` command [#121](https://github.com/arduino/arduino-app-cli/pull/121)
- Remove `skip-python` from the HTTP  `POST /v1/apps` request endpoint [#121](https://github.com/arduino/arduino-app-cli/pull/121)




### Fixed

- pkg/board/remote/adb/: use `adb get-state` for checking if the board is there and it is connected [#98](https://github.com/arduino/arduino-app-cli/pull/98)
- add the `/needrestart/conf.d/arduino-adbd.conf` to exclude `adbd` service to be restarted [#117](https://github.com/arduino/arduino-app-cli/pull/117)
- remove `compatible_models` from the `GET /v1/apps/{id}/bricks` endpoint [#120](https://github.com/arduino/arduino-app-cli/pull/120)
- manage missing sketch folder when adding a library [#123](https://github.com/arduino/arduino-app-cli/pull/123)

### Security

## [0.6.8] - 2025-11-24

### Added

- Add `arduino-avahi-serial.service` systemd service that runs once to add the `serial_number` to the avahi daemon [#48](https://github.com/arduino/arduino-app-cli/pull/48)
- Add the `daemon_version` field to the `arduino-app-cli version` command [#49](https://github.com/arduino/arduino-app-cli/pull/49)
- Refactor README.md and contribution guidelines [#58](https://github.com/arduino/arduino-app-cli/pull/58)
- Add `arduino-app-cli app clean-cache <app-id>` command [#59](https://github.com/arduino/arduino-app-cli/pull/59)
- Improve the help message of `arduino-app-cli` [#61](https://github.com/arduino/arduino-app-cli/pull/61)
- Add `code` field with `NO_INTERNET_CONNECTION`, `OPERATION_IN_PROGRESS` and `UNKNOWN_ERROR` values in the update handlers and CLI [#62](https://github.com/arduino/arduino-app-cli/pull/62)
- pkg/board/board: expose the `serial_number` in network mode [#63](https://github.com/arduino/arduino-app-cli/pull/63)
- pkg/board/remote/adb: return `failed to create command` error when `GetCmd` fails [#79](https://github.com/arduino/arduino-app-cli/pull/79)
- Update `arduino-cli` dependencies [#86](https://github.com/arduino/arduino-app-cli/pull/86)
- Return detailed `error running needrestart command` error information during updates [#95](https://github.com/arduino/arduino-app-cli/pull/95)

### Removed

- Remove `arduino-app-cli app ps` command [#65](https://github.com/arduino/arduino-app-cli/pull/65)

### Fixed

- Allow missing required variables when a brick is added [#74](https://github.com/arduino/arduino-app-cli/pull/74)
- pkg/board/remote/adb: fix zombie process in adb that resulted in `fork failed: resource temporarily unavailable` error [#81](https://github.com/arduino/arduino-app-cli/pull/81)
- Check if the app is running/starting before stopping it in `arduino-app-cli app stop` and `DELETE /v1/apps/{appID}` [#84](https://github.com/arduino/arduino-app-cli/pull/84)
- Early return without opening SSE stream when `HEAD /system/update/events` is called [#96](https://github.com/arduino/arduino-app-cli/pull/96)

## [0.6.7] - 2025-11-10

### Added

- Flash sketch in ram [#12](https://github.com/arduino/arduino-app-cli/pull/12)
- Return `config_variables` field in `/apps/:id/bricks` and `apps/:id/bricks/:id` [#18](https://github.com/arduino/arduino-app-cli/pull/18)
- Add brick details completions [#54](https://github.com/arduino/arduino-app-cli/pull/54)

### Fixed

- Remove the `variable default value cannot be empty` error when adding a brick to an app [#44](https://github.com/arduino/arduino-app-cli/pull/44)
- Install libraries missing from local library-index [#50](https://github.com/arduino/arduino-app-cli/pull/50)

## [0.6.6] - 2025-11-03

### Added

- Add `used_by_apps` field to the `GET /v1/bricks/{brickID}` [#30](https://github.com/arduino/arduino-app-cli/pull/30)
- Improve `arduino-app-cli restart` command [#37](https://github.com/arduino/arduino-app-cli/pull/37)
- Return adb stdout/err in case of error [#40](https://github.com/arduino/arduino-app-cli/pull/40)

### Fixed

- Remove `Requires` from the systemd `arduino-app-cli.service` [#34](https://github.com/arduino/arduino-app-cli/pull/34)
- Fix websocket origin validation (fixes serial monitor on Windows) [#39](https://github.com/arduino/arduino-app-cli/pull/39)
- Use a valid origin in `GET /v1/monitor/ws` [#41](https://github.com/arduino/arduino-app-cli/pull/41)

## [0.6.5] - 2025-10-27

### Removed

- Remove the `arduino-app-cli board` sub-command [#27](https://github.com/arduino/arduino-app-cli/pull/27)
- Remove the internal zephyr core [#28](https://github.com/arduino/arduino-app-cli/pull/28)

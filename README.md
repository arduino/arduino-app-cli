# Arduino App CLI

`arduino-app-cli` is a command line tool running on the [Arduino UNO Q](https://docs.arduino.cc/hardware/uno-q/) boards, that manages and runs Arduino Apps (both Linux and microcontroller parts), provides a HTTP daemon mode to expose RestFul APIs, and auto-updates itself and other components.

[![Test Go status](https://github.com/arduino/arduino-app-cli/actions/workflows/go-test.yml/badge.svg)](https://github.com/arduino/arduino-app-cli/actions/workflows/go-test.yml)

## Docs

For guidance on installation and development, see the [User documentation].

## Dependencies

`arduino-app-cli` drives model downloads through the `models-downloader` container from
[app-bricks-py], declared per handler in `models-handlers.yaml`, which ships in the
app-bricks wheel the `RUNNER_VERSION` in `Taskfile.yml` pins. The Go side reads what that
container prints, so some features need a minimum image:

| Feature                                                      | Needs                                                                                   | Why                                                                                                                                                                                                               |
| ------------------------------------------------------------ | --------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /v1/models`, which downloads a model no entry declares | a `models-downloader` reporting `model_id` on its download events ([app-bricks-py#416]) | The downloader names the model after the file that arrives. Without that field the model installs, but the endpoint cannot say which one it is, so the stream ends with an error event instead of the model.      |
| Listing a model no `models-list.yaml` entry declares         | a `models-downloader` reporting `model_origin` and `download_metadata` in its listing   | An undeclared model is reconstructed from its on-disk record. An older image reports neither, so nothing is appended and only declared models are listed.                                                         |
| Running an undeclared model, once installed                  | `llamacpp-runner` and `llamacpp-npu-runner` from the same release ([app-bricks-py#415]) | The runners regenerate `models.ini` at startup. An older one sections an undeclared model by its bare filename while its id carries the repository path, so `arduino:llm` cannot find it among the served models. |

## Quickstart

// TODO

## How to contribute

Contributions are welcome!

Please read the [Contributor Guide] document, which will show you how to build the source code, run the tests, and
contribute your changes to the project.

:sparkles: Thanks to all our [contributors]! :sparkles:

## Security

If you think you have found a vulnerability or other security-related bug in the Arduino CLI, please read our [security
policy] and report the bug to our Security Team 🛡️ Thank you!

e-mail contact: security@arduino.cc

## License

GPL-3.0-or-later

[app-bricks-py]: https://github.com/arduino/app-bricks-py
[app-bricks-py#415]: https://github.com/arduino/app-bricks-py/pull/415
[app-bricks-py#416]: https://github.com/arduino/app-bricks-py/pull/416
[user documentation]: docs/user-documentation.md
[contributor guide]: docs/CONTRIBUTING.md
[security policy]: https://github.com/arduino/arduino-app-cli/security/policy
[contributors]: https://github.com/arduino/arduino-app-cli/graphs/contributors

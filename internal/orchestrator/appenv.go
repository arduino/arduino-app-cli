// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/linuxconfig"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

const appHomeRef = "${APP_HOME}"

// An app runs as uid 1000, the same the cli itself requires (cmd/arduino-app-cli),
// in the arduino group, which is required to exist but not to have a known id.
const appUserRef = "1000:${ARDUINO_GID}"

// AppEnv is the environment an app runs with, in the two halves the provisioning
// is split in: BuildTime values are baked into the generated compose files, while
// Runtime values are resolved on the host at every start and only referenced
// there as ${VAR}.
type AppEnv struct {
	buildTime types.Mapping
	runtime   types.Mapping
}

// All is the environment docker compose runs with. Both halves are needed: the
// brick and service compose files being included interpolate build-time values
// too. Merge only adds what is not set yet, so the runtime values win.
func (e AppEnv) All() types.Mapping {
	return e.runtime.Clone().Merge(e.buildTime)
}

// ForComposeFile is the `environment:` section to write in the generated compose
// files: the build-time values, plus the runtime keys as ${VAR} references so the
// file declares which variables the app is wired with while their values stay in
// the app env file.
func (e AppEnv) BuildTime() types.Mapping {
	env := e.buildTime.Clone()
	// Referenced, not resolved: docker compose fills these in at start time from
	// the app env file, so the file states which variables the app is wired with
	// without stating anything about the host it was generated on.
	for key := range e.runtime {
		env[key] = "${" + key + "}"
	}
	// A brick can declare a VIDEO_DEVICE of its own, and it has to keep applying
	// when there is no camera to detect on the board.
	if brickValue, ok := e.buildTime["VIDEO_DEVICE"]; ok {
		env["VIDEO_DEVICE"] = fmt.Sprintf("${VIDEO_DEVICE:-%s}", brickValue)
	}
	return env
}

// AppEnvironment resolves both halves of the app environment.
func AppEnvironment(
	ctx context.Context,
	arduinoApp app.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
	modelsIndex *modelsindex.ModelsIndex,
	plat platform.Platform,
	cfg config.Configuration,
) AppEnv {
	return AppEnv{
		buildTime: buildTimeAppEnv(ctx, arduinoApp, bricksIndex, modelsIndex, plat),
		runtime:   runtimeAppEnv(ctx, arduinoApp.FullPath, cfg),
	}
}

func buildTimeAppEnv(
	ctx context.Context,
	app app.ArduinoApp,
	brickIndex *bricksindex.BricksIndex,
	modelsIndex *modelsindex.ModelsIndex,
	plat platform.Platform,
) types.Mapping {
	envs := make(types.Mapping)

	brickIndex = brickIndex.WithAppBricks(app.LocalBricks)

	for _, brick := range app.Descriptor.Bricks {
		if brickDef, found := brickIndex.FindBrickByID(brick.ID); found {
			maps.Insert(envs, brickDef.GetDefaultVariables())
		}

		if m, err := modelsIndex.GetModelByID(ctx, brick.Model); err != nil {
			slog.Warn("unable to get model for brick", slog.String("brickID", brick.ID), slog.String("modelID", brick.Model), slog.String("error", err.Error()))
		} else if m != nil {
			for _, b := range m.Bricks {
				maps.Insert(envs, maps.All(b.ModelConfiguration))
			}
		}

		slog.Debug("adding Brick", slog.String("brickID", brick.ID), slog.String("model", brick.Model), slog.Any("variables", brick.Variables))
		maps.Insert(envs, maps.All(brick.Variables))
	}

	envs["BOARD_NAME"] = plat.BoardName

	slog.Debug("Build-time environment variables", slog.Any("envs", envs))

	return envs
}

// runtimeAppEnv resolves the host facts the generated compose files reference as
// ${VAR}. All the keys are always set, empty when the host has nothing to offer,
// so that the generated files and the app env file do not change shape depending
// on what is plugged in the board.
func runtimeAppEnv(ctx context.Context, appPath *paths.Path, cfg config.Configuration) types.Mapping {
	envs := make(types.Mapping, 6)

	envs["APP_HOME"] = appPath.String()
	envs["VIDEO_DEVICE"] = ""
	envs["CONFIGURED_CARRIERS"] = ""
	envs["HOST_IP"] = ""
	// Directory where AI models are installed, shared with the containerized runners.
	envs["MODELS_PATH"] = cfg.ModelsDir().String()
	envs["XDG_RUNTIME_DIR"] = "/run/user/1000"
	// Primary group of the app, so the files it writes stay readable by the cli.
	envs["ARDUINO_GID"] = func() string {
		if group, err := user.LookupGroup("arduino"); err == nil {
			return group.Gid
		}
		slog.Warn("arduino group not found on host; using the group of the current process")
		return strconv.Itoa(os.Getgid())
	}()

	// Pre-select default camera device if available. This can be overridden by the app environment variables (or in future by applab)
	// This is required because there are some video devices for HW acceleration that are auto registered in /dev but are not real cameras.
	if videoDevices := peripherals.GetVideoDevices(); len(videoDevices) > 0 {
		// VIDEO_DEVICE will be the first device in /dev/v4l/by-id
		envs["VIDEO_DEVICE"] = videoDevices[0]
	}

	mediaCarriers, err := linuxconfig.GetEnabledCarriers(ctx)
	if err != nil {
		slog.Warn("unable to get configured carriers", slog.String("error", err.Error()))
	} else if len(mediaCarriers) > 0 {
		carrierNames := f.Map(mediaCarriers, func(c linuxconfig.Carrier) string {
			return c.CarrierName
		})
		envs["CONFIGURED_CARRIERS"] = strings.Join(carrierNames, ",")
	}

	if hostIP, err := helpers.GetHostIP(); err == nil {
		envs["HOST_IP"] = hostIP
	} else {
		slog.Warn("unable to get host IP", slog.String("error", err.Error()))
	}

	slog.Debug("Runtime environment variables", slog.Any("envs", envs))

	return envs
}

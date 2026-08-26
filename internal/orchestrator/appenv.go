// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"log/slog"
	"maps"
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
const appUserExpr = `{{ with groupID "arduino" }}1000:{{ . }}{{ else }}1000{{ end }}`

// hostVariables are the host facts a template references as ${VAR}: the resolve
// step writes the references, hostEnvironment answers them.
var hostVariables = []string{
	"APP_HOME",
	"VIDEO_DEVICE",
	"CONFIGURED_CARRIERS",
	"HOST_IP",
	"MODELS_PATH",
	"XDG_RUNTIME_DIR",
}

// appEnvironment is what the app is wired with, from the app and the board it is built
// for: nothing here is read off a machine.
func appEnvironment(
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

// hostEnvironment answers hostVariables on this board. Every one is always set, so
// a template does not change shape with what is plugged in the board.
func hostEnvironment(ctx context.Context, appPath *paths.Path, cfg config.Configuration) types.Mapping {
	envs := make(types.Mapping, len(hostVariables))
	for _, name := range hostVariables {
		envs[name] = ""
	}

	envs["APP_HOME"] = appPath.String()
	// Directory where AI models are installed, shared with the containerized runners.
	envs["MODELS_PATH"] = cfg.ModelsDir().String()
	envs["XDG_RUNTIME_DIR"] = "/run/user/1000"

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

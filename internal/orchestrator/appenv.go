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

// hostVariables are what a template references as ${VAR}: the name is the reference the
// resolve step writes, the function is the answer the render step fills in.
var hostVariables = map[string]func(hostFacts) string{
	"APP_HOME":        func(facts hostFacts) string { return facts.appPath.String() },
	"XDG_RUNTIME_DIR": func(hostFacts) string { return "/run/user/1000" },
	// Directory where AI models are installed, shared with the containerized runners.
	"MODELS_PATH": func(facts hostFacts) string { return facts.cfg.ModelsDir().String() },

	// Some video devices in /dev are for HW acceleration and are not real cameras, so
	// the first one in /dev/v4l/by-id is picked. A brick can override it.
	"VIDEO_DEVICE": func(hostFacts) string {
		if videoDevices := peripherals.GetVideoDevices(); len(videoDevices) > 0 {
			return videoDevices[0]
		}
		return ""
	},

	"CONFIGURED_CARRIERS": func(facts hostFacts) string {
		carriers, err := linuxconfig.GetEnabledCarriers(facts.ctx)
		if err != nil {
			slog.Warn("unable to get configured carriers", slog.String("error", err.Error()))
			return ""
		}
		return strings.Join(f.Map(carriers, func(c linuxconfig.Carrier) string { return c.CarrierName }), ",")
	},

	"HOST_IP": func(hostFacts) string {
		hostIP, err := helpers.GetHostIP()
		if err != nil {
			slog.Warn("unable to get host IP", slog.String("error", err.Error()))
			return ""
		}
		return hostIP
	},
}

// hostFacts is what the answers above are allowed to look at, besides the board.
type hostFacts struct {
	ctx     context.Context
	appPath *paths.Path
	cfg     config.Configuration
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
	isSecret := secretNames(app, brickIndex)

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

		// A secret value is never logged, whether it comes from the store or is
		// still in app.yaml.
		logged := maps.Clone(brick.Variables)
		for name := range logged {
			if isSecret[name] {
				logged[name] = "***"
			}
		}
		slog.Debug("adding Brick", slog.String("brickID", brick.ID), slog.String("model", brick.Model), slog.Any("variables", logged))
		maps.Insert(envs, maps.All(brick.Variables))
	}

	envs["BOARD_NAME"] = plat.BoardName

	// A secret is only referenced here: its value is filled in when the app is
	// rendered, so it is never written to a template that can be shipped.
	for name := range isSecret {
		envs[name] = "${" + name + "}"
	}

	slog.Debug("Build-time environment variables", slog.Any("envs", envs))

	return envs
}

// hostEnvironment answers every hostVariable on this board. Each is always set, even
// when empty, so a template does not change shape with what is plugged in the board.
func hostEnvironment(ctx context.Context, appPath *paths.Path, cfg config.Configuration) types.Mapping {
	facts := hostFacts{ctx: ctx, appPath: appPath, cfg: cfg}

	envs := make(types.Mapping, len(hostVariables))
	for name, answer := range hostVariables {
		envs[name] = answer(facts)
	}

	slog.Debug("Host environment variables", slog.Any("envs", envs))

	return envs
}

// secretNames are the variables the bricks of an app declare secret.
func secretNames(arduinoApp app.ArduinoApp, brickIndex *bricksindex.BricksIndex) map[string]bool {
	brickIndex = brickIndex.WithAppBricks(arduinoApp.LocalBricks)

	names := map[string]bool{}
	for _, brick := range arduinoApp.Descriptor.Bricks {
		brickDef, found := brickIndex.FindBrickByID(brick.ID)
		if !found {
			continue
		}
		for _, variable := range brickDef.Variables {
			if variable.Secret {
				names[variable.Name] = true
			}
		}
	}
	return names
}

// appSecrets is the value of every variable a brick declares secret, read on the board
// the app runs on and never written to a template.
//
// A value in app.yaml wins, because the api still writes there and secrets.Adopt has
// copied it into the store already. A release has none, so the store answers for it.
func appSecrets(arduinoApp app.ArduinoApp, brickIndex *bricksindex.BricksIndex, stored map[string]string) types.Mapping {
	brickIndex = brickIndex.WithAppBricks(arduinoApp.LocalBricks)

	secrets := make(types.Mapping)
	for _, brick := range arduinoApp.Descriptor.Bricks {
		brickDef, found := brickIndex.FindBrickByID(brick.ID)
		if !found {
			continue
		}
		for _, variable := range brickDef.Variables {
			if !variable.Secret {
				continue
			}
			secrets[variable.Name] = variable.DefaultValue
			if value := stored[variable.Name]; value != "" {
				secrets[variable.Name] = value
			}
			if value, set := brick.Variables[variable.Name]; set && value != "" {
				secrets[variable.Name] = value
			}
		}
	}
	return secrets
}

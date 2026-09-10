// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// PrepareRelease downloads what an installed release needs to run: the containers its
// frozen compose file names and the models it is wired with. A start downloads both
// anyway, so this only moves the wait to the install, where a board is still attended.
func PrepareRelease(
	ctx context.Context,
	docker command.Cli,
	arduinoApp app.ArduinoApp,
	cfg config.Configuration,
	plat platform.Platform,
	cb func(StreamMessage),
) error {
	if cb == nil {
		cb = func(StreamMessage) {}
	}
	if _, isRelease := arduinoApp.GetRelease(); !isRelease {
		return fmt.Errorf("%w: %q is not installed from a release", ErrBadRequest, arduinoApp.Name)
	}

	// The containers first: they are the same for every board, so a failure here is not
	// worth waiting for a model to find out.
	cb(StreamMessage{data: "downloading the containers"})
	cb(StreamMessage{progress: &Progress{Name: "containers", Progress: 0.0}})
	if err := pullReleaseImages(ctx, arduinoApp, cb); err != nil {
		return fmt.Errorf("failed to download the containers of the release: %w", err)
	}

	manifest, err := readReleaseManifest(arduinoApp.FullPath)
	if err != nil {
		return err
	}
	if len(manifest.Models) == 0 {
		cb(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
		return nil
	}

	// The release states its models, and ships their records: the index of this board is
	// not the one that built it and may not know them.
	models, err := frozenModelsIndex(arduinoApp, docker, cfg, plat)
	if err != nil {
		return fmt.Errorf("cannot read the models the release ships: %w", err)
	}
	for i, model := range manifest.Models {
		done := 50.0 + 50.0*float32(i)/float32(len(manifest.Models))
		cb(StreamMessage{data: "downloading the model " + model.ID})
		cb(StreamMessage{progress: &Progress{Name: "models", Progress: done}})
		if _, err := models.Install(ctx, docker, model.ID, plat, func(message modelsindex.StreamMessage) {
			if message.IsData() {
				cb(StreamMessage{data: message.GetData()})
			}
		}); err != nil {
			return fmt.Errorf("failed to download the model %q: %w", model.ID, err)
		}
	}

	cb(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	return nil
}

// renderRelease writes the compose file docker is given, from the templates the release
// froze. It runs once the release is in place: the paths it resolves are absolute, so a
// staging dir would be the one baked in.
func renderRelease(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	arduinoApp app.ArduinoApp,
	cfg config.Configuration,
	plat platform.Platform,
) error {
	bricksIndex, err := arduinoApp.ReleaseBricks()
	if err != nil {
		return fmt.Errorf("cannot read the bricks the release ships: %w", err)
	}
	modelsIndex, err := frozenModelsIndex(arduinoApp, docker, cfg, plat)
	if err != nil {
		return fmt.Errorf("cannot read the models the release ships: %w", err)
	}

	// The secrets are empty until they are set on this board, which the render does not
	// need: an image name is frozen and no host fact depends on one.
	appEnv := appEnvironment(ctx, arduinoApp, bricksIndex, modelsIndex, plat)
	env := hostEnvironment(ctx, arduinoApp.FullPath, cfg).Merge(appEnv)
	if err := provisioner.Render(ctx, &arduinoApp, env, appSecrets(arduinoApp, bricksIndex)); err != nil {
		return fmt.Errorf("failed to render the compose file of the release: %w", err)
	}
	return nil
}

// pullReleaseImages hands the rendered compose file to docker, which is what knows the
// images it names and what is already on the board.
func pullReleaseImages(ctx context.Context, arduinoApp app.ArduinoApp, cb func(StreamMessage)) error {
	commands := []string{"docker", "compose", "-f", arduinoApp.AppComposeFilePath().String(), "pull"}

	dockerParser := NewDockerProgressParser(200)
	writer := NewCallbackWriter(func(line string) {
		if percentage, ok := dockerParser.Parse(line); ok {
			// The containers are the first half of a prepare, the models the second.
			cb(StreamMessage{progress: &Progress{Name: "containers", Progress: float32(percentage / 2.0)}})
			return
		}
		cb(StreamMessage{data: line})
	})

	process, err := paths.NewProcess(nil, commands...)
	if err != nil {
		return err
	}
	process.RedirectStderrTo(writer)
	process.RedirectStdoutTo(writer)
	return process.RunWithinContext(ctx)
}

// frozenModelsIndex is the models index a release ships in its .cache. The models land
// where every model does, so what is downloaded here is what a start reads.
func frozenModelsIndex(
	arduinoApp app.ArduinoApp,
	docker command.Cli,
	cfg config.Configuration,
	plat platform.Platform,
) (*modelsindex.ModelsIndex, error) {
	return modelsindex.Load(
		plat,
		arduinoApp.ProvisioningStateDir(),
		cfg.ModelsDir(),
		cfg.CustomModelsDir(),
		docker.Client(),
		cfg,
	)
}

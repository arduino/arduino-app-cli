// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// PrepareRelease downloads what an installed release needs to run: the containers its
// frozen compose file names and the models it is wired with.
func PrepareRelease(
	ctx context.Context,
	docker command.Cli,
	arduinoApp app.ArduinoApp,
	prj *types.Project,
	cfg config.Configuration,
	plat platform.Platform,
	cb func(StreamMessage),
) error {
	if cb == nil {
		cb = func(StreamMessage) {}
	}
	if !arduinoApp.IsRelease() {
		return fmt.Errorf("%w: %q is not installed from a release", ErrBadRequest, arduinoApp.Name)
	}

	// The containers first: a failure here is not worth waiting for a model to find out.
	cb(StreamMessage{data: "downloading the containers"})
	cb(StreamMessage{progress: &Progress{Name: "containers", Progress: 0.0}})
	if err := dockerhelper.PullImages(ctx, docker.Client(), dockerhelper.ComposeImages(prj),
		func(line string) { cb(StreamMessage{data: line}) },
		func(label string, curr, total int64) {
			// The containers are the first half of a prepare, the models the second.
			cb(StreamMessage{progress: &Progress{Name: label, Progress: float32(curr * 50 / total)}})
		}); err != nil {
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

// PrepareInstalledRelease renders the compose project of an already installed release
// and downloads what it needs to run, without starting it: the same rendering an
// install does, followed by the prepare part alone.
func PrepareInstalledRelease(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	arduinoApp app.ArduinoApp,
	cfg config.Configuration,
	plat platform.Platform,
	cb func(StreamMessage),
) error {
	if !arduinoApp.IsRelease() {
		return fmt.Errorf("%w: %q is not installed from a release", ErrBadRequest, arduinoApp.Name)
	}

	prj, err := renderRelease(ctx, docker, provisioner, arduinoApp, cfg, plat)
	if err != nil {
		return err
	}
	return PrepareRelease(ctx, docker, arduinoApp, prj, cfg, plat, cb)
}

// renderRelease writes the compose file docker is given, from the templates the release
// froze. It runs once the release is in place: the paths it resolves are absolute.
func renderRelease(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	arduinoApp app.ArduinoApp,
	cfg config.Configuration,
	plat platform.Platform,
) (*types.Project, error) {
	bricksIndex, err := arduinoApp.ReleaseBricks()
	if err != nil {
		return nil, fmt.Errorf("cannot read the bricks the release ships: %w", err)
	}
	modelsIndex, err := frozenModelsIndex(arduinoApp, docker, cfg, plat)
	if err != nil {
		return nil, fmt.Errorf("cannot read the models the release ships: %w", err)
	}

	// The secrets are empty until they are set on this board, which the render does not
	// need: an image name is frozen and no host fact depends on one.
	appEnv := appEnvironment(ctx, arduinoApp, bricksIndex, modelsIndex, plat)
	env := hostEnvironment(ctx, arduinoApp.FullPath, cfg).Merge(appEnv)
	prj, err := provisioner.Render(ctx, &arduinoApp, env, arduinoApp.Secrets(bricksIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to render the compose file of the release: %w", err)
	}
	return prj, nil
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

// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

// The servicelocator pkg should be used only under cmd/arduino-app-cli as a convenience to build our DI.

package servicelocator

import (
	"sync"

	dockerCommand "github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	dockerClient "github.com/docker/docker/client"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/store"
)

var globalConfig config.Configuration

func Init(cfg config.Configuration) {
	globalConfig = cfg
}

var (
	GetBricksIndex = sync.OnceValue(func() *bricksindex.BricksIndex {
		return f.Must(bricksindex.GenerateBricksIndexFromFile(GetStaticStore().GetAssetsFolder()))
	})

	GetModelsIndex = sync.OnceValue(func() *modelsindex.ModelsIndex {
		return f.Must(modelsindex.GenerateModelsIndexFromFile(GetStaticStore().GetAssetsFolder()))
	})

	GetProvisioner = sync.OnceValue(func() *orchestrator.Provision {
		return f.Must(orchestrator.NewProvision(
			GetDockerClient(),
			globalConfig,
		))
	})

	docker *dockerCommand.DockerCli

	GetDockerClient = sync.OnceValue(func() *dockerCommand.DockerCli {
		docker = f.Must(dockerCommand.NewDockerCli(
			dockerCommand.WithAPIClient(
				f.Must(dockerClient.NewClientWithOpts(
					dockerClient.FromEnv,
					dockerClient.WithAPIVersionNegotiation(),
				)),
			),
		))
		if err := docker.Initialize(flags.NewClientOptions()); err != nil {
			panic(err)
		}
		return docker
	})

	CloseDockerClient = func() error {
		if docker != nil {
			return docker.Client().Close()
		}
		return nil
	}

	GetStaticStore = sync.OnceValue(func() *store.StaticStore {
		return store.NewStaticStore(globalConfig.AssetsDir().Join(globalConfig.UsedPythonImageTag).String())
	})

	GetBrickService = sync.OnceValue(func() *bricks.Service {
		return bricks.NewService(
			GetModelsIndex(),
			GetBricksIndex(),
			GetStaticStore(),
		)
	})

	GetAppIDProvider = sync.OnceValue(func() *app.IDProvider {
		return app.NewAppIDProvider(globalConfig)
	})
)

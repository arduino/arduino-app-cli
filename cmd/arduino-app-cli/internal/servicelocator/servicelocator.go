// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/store"
)

var globalConfig config.Configuration

func Init(cfg config.Configuration) {
	globalConfig = cfg
}

var (
	GetBricksIndex = sync.OnceValue(func() *bricksindex.BricksIndex {
		return f.Must(bricksindex.Load(GetStaticStore().GetAssetsFolder()))
	})

	GetModelsIndex = sync.OnceValue(func() *modelsindex.ModelsIndex {
		return f.Must(modelsindex.Load(GetStaticStore().GetAssetsFolder(), globalConfig.CustomModelsDir()))
	})

	GetServicesIndex = sync.OnceValue(func() *servicesindex.ServicesIndex {
		return f.Must(servicesindex.Load(GetStaticStore().GetServicesFolder()))
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
		)
	})

	GetAppIDProvider = sync.OnceValue(func() *app.IDProvider {
		return app.NewAppIDProvider(globalConfig)
	})

	GetPlatform = sync.OnceValue(func() platform.Platform {
		return platform.GetPlatform(globalConfig.DataDir())
	})
)

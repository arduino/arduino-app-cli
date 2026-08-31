// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"fmt"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
)

type PortInfo struct {
	Port            string `json:"port"`
	Source          string `json:"source"`
	RequiresDisplay string `json:"-"`
}

// AppPorts returns every host port published by the app: the ports declared by its app.yaml, the
// ones declared by its bricks (in the brick index and in the brick compose files), and the ones
// published by the services the bricks require.
func AppPorts(
	a app.ArduinoApp,
	index *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
) ([]PortInfo, error) {
	index = index.WithAppBricks(a.LocalBricks)

	ports := make([]PortInfo, 0, len(a.Descriptor.Ports)+len(a.Descriptor.Bricks))

	for _, p := range a.Descriptor.Ports {
		ports = append(ports, PortInfo{
			Port:   strconv.Itoa(p),
			Source: appPortsSource,
		})
	}

	for _, appBrick := range a.Descriptor.Bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			return nil, fmt.Errorf("brick %q not found in the index", appBrick.ID)
		}
		for _, p := range indexBrick.GetPorts() {
			ports = append(ports, PortInfo{
				Port:            p,
				Source:          appBrick.ID,
				RequiresDisplay: indexBrick.RequiresDisplay,
			})
		}
	}

	services, err := RequiredServices(a.Descriptor.Bricks, index, servicesIndex)
	if err != nil {
		return nil, err
	}
	for _, service := range services {
		for _, p := range service.Ports {
			ports = append(ports, PortInfo{
				Port:   p,
				Source: service.ID,
			})
		}
	}

	return ports, nil
}

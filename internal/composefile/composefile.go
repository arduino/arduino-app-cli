// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package composefile provides helpers to inspect Docker Compose files without
// contacting the Docker daemon.
package composefile

import (
	"context"
	"fmt"
	"os"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
)

// ExtractPorts returns the host ports published by every service declared in the given
// compose file. When a service does not publish a port explicitly, the target port is
// reported instead.
func ExtractPorts(composeFile *paths.Path) ([]string, error) {
	content, err := composeFile.ReadFile()
	if err != nil {
		return nil, err
	}

	prj, err := loader.LoadWithContext(
		context.Background(),
		types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Content: content}},
			Environment: types.NewMapping(os.Environ()),
		},
		func(o *loader.Options) { o.SetProjectName("default", false); o.SkipConsistencyCheck = true },
		loader.WithSkipValidation,
	)
	if err != nil {
		return nil, err
	}

	var ports []string
	for _, svc := range prj.Services {
		for _, p := range svc.Ports {
			if p.Published != "" {
				ports = append(ports, p.Published)
			} else {
				ports = append(ports, fmt.Sprintf("%d", p.Target))
			}
		}
	}
	return ports, nil
}

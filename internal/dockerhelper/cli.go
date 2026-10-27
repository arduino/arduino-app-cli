// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !js

package dockerhelper

import (
	"github.com/docker/cli/cli/command"
	"github.com/moby/moby/client"
)

// Cli is the docker client the rest of the project is handed, and EngineClient the
// engine's api it gives. On js/wasm, where there is no docker, they are types of this
// package (cli_js.go), so that nothing outside it names docker's.
type (
	Cli          = command.Cli
	EngineClient = client.APIClient
)

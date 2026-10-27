// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

package orchestrator

// The logs of an app where there is no docker, as in a browser: no app runs there.

import (
	"context"
	"iter"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
)

func AppLogs(ctx context.Context, app app.ArduinoApp, req AppLogsRequest, dockerCli dockerhelper.Cli, bricksIndex *bricksindex.BricksIndex, servicesIndex *servicesindex.ServicesIndex, cfg config.Configuration) (r0 iter.Seq[LogMessage], err error) {
	err = dockerhelper.ErrNoDocker
	return
}

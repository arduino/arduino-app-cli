// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

type AppLogsRequest struct {
	ShowAppLogs      bool
	ShowServicesLogs bool
	Follow           bool
	Tail             *uint64
}

type LogSource string

const (
	LogSourceMain  LogSource = "main"
	LogSourceBrick LogSource = "brick"
)

type LogMessage struct {
	Source        LogSource
	BrickID       string // empty when Source != LogSourceBrick
	ContainerName string
	Content       string
}

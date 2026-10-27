// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

package dockerhelper

// NoDocker is the docker client, and the engine, of a place with no docker, such as a
// browser: there every Cli and EngineClient is one, and nil works as well as any.
type NoDocker struct{}

type (
	Cli          = *NoDocker
	EngineClient = *NoDocker
)

// Client is the engine's api, which is no engine either.
func (*NoDocker) Client() EngineClient { return nil }

// Close has nothing to close.
func (*NoDocker) Close() error { return nil }

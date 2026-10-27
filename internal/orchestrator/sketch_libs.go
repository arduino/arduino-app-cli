// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"time"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
)

const indexUpdateInterval = 10 * time.Minute

func rpcProfileLibReferenceToLibReleaseID(ref *rpc.ProfileLibraryReference) LibraryReleaseID {
	l := ref.GetIndexLibrary()
	return LibraryReleaseID{
		Name:         l.GetName(),
		Version:      l.GetVersion(),
		IsDependency: l.GetIsDependency(),
	}
}

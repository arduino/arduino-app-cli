// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package apprelease

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// NewReleaseCmd builds the `release` command group, which manages reproducible Arduino App
// Releases: self-contained .tar.gz bundles that include the prebuilt sketch, the user code,
// the provisioned Python venv, the required AI models and a version-pinned compose file. A
// release can be installed on another board without recompiling the sketch or downloading
// Python dependencies (only containers are pulled).
func NewReleaseCmd(cfg config.Configuration, cliVersion string) *cobra.Command {
	releaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Build, install, prepare and clone reproducible Arduino App Releases",
		Long: `Manage reproducible Arduino App Releases.

An Arduino App Release is a self-contained archive of an already-built app: it bundles the
compiled sketch binary, the user code, the provisioned Python virtual environment, the
required AI models and a version-pinned Docker Compose file. Once installed, a release
behaves like a regular app and is launched with the usual 'app start' command; it is not
recompiled nor re-provisioned, and only containers are pulled.`,
	}

	releaseCmd.AddCommand(newBuildCmd(cfg, cliVersion))
	releaseCmd.AddCommand(newInstallCmd(cfg))
	releaseCmd.AddCommand(newPrepareCmd(cfg))
	releaseCmd.AddCommand(newCloneCmd(cfg))

	return releaseCmd
}

// loadApp resolves an app id-or-path to an ArduinoApp, mirroring the helper used by the
// `app` command group.
func loadApp(idOrPath string) (app.ArduinoApp, error) {
	id, err := servicelocator.GetAppIDProvider().ParseID(idOrPath)
	if err != nil {
		return app.ArduinoApp{}, fmt.Errorf("invalid app path: %s", idOrPath)
	}
	return app.Load(id.ToPath())
}

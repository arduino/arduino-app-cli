// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package cmdutil

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestIDToAlias(t *testing.T) {
	t.Run("empty id returns empty string", func(t *testing.T) {
		require.Equal(t, "", IDToAlias(appid.ID{}))
	})

	t.Run("prefixed id is returned as is", func(t *testing.T) {
		tmpDir := paths.New(t.TempDir())
		t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmpDir.Join("apps").String())
		t.Setenv("ARDUINO_APP_CLI__CONFIG_DIR", tmpDir.Join("config").String())
		t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmpDir.Join("data").String())
		cfg, err := config.NewFromEnv()
		require.NoError(t, err)

		id, err := appid.NewAppProvider(cfg, platform.Platform{BoardName: "unoq"}).ParseID("user:my-app")
		require.NoError(t, err)
		require.Equal(t, "user:my-app", IDToAlias(id))
	})
}

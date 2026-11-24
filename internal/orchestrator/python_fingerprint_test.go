// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func TestThePythonFingerprintMatchesNothingOnceForgotten(t *testing.T) {
	arduinoApp := app.ArduinoApp{FullPath: paths.New(t.TempDir())}
	python := pythonFingerprint(arduinoApp)
	hash := hashOf([]byte("services: {main: {image: python-apps-base}}"))

	require.NoError(t, python.forget(), "nothing was saved yet")

	require.NoError(t, python.save(hash))
	assert.True(t, python.matches(hash))
	assert.FileExists(t, arduinoApp.ProvisioningStateDir().Join("python", "fingerprint.json").String())

	require.NoError(t, python.forget())
	assert.False(t, python.matches(hash), "a start that failed leaves no record")
}

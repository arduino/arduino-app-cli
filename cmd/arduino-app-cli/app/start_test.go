// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func TestStartErrorMessageSuggestsConfiguringMissingVariables(t *testing.T) {
	err := errors.Join(
		&orchestrator.MissingRequiredVariableError{
			Name:    "ARDUINO_DEVICE_ID",
			BrickID: "arduino:arduino_cloud",
		},
		&orchestrator.MissingRequiredVariableError{
			Name:    "ARDUINO_SECRET",
			BrickID: "arduino:arduino_cloud",
		},
	)

	require.Equal(t, `[ERROR] variable "ARDUINO_DEVICE_ID" is required by brick "arduino:arduino_cloud"
variable "ARDUINO_SECRET" is required by brick "arduino:arduino_cloud"
To configure the variables use:
  arduino-app-cli app brick config /apps/weather arduino:arduino_cloud ARDUINO_DEVICE_ID=value
  arduino-app-cli app brick config /apps/weather arduino:arduino_cloud ARDUINO_SECRET=value`, decorateErrorMessage(err, "/apps/weather"))
}

func TestStartErrorMessageDoesNotSuggestConfigForOtherErrors(t *testing.T) {
	require.Equal(t, `[ERROR] app "other" is running`, decorateErrorMessage(errors.New(`app "other" is running`), "/apps/weather"))
}

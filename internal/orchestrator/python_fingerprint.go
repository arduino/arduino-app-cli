// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

// The Python part of an app is the compose project of its containers. What it is built from
// is the compose file rendered for this board, which names the images, the bricks, the
// ports and the environment of the project. Its fingerprint is saved once compose brought
// that project up, so it tells whether the app folder still describes what runs.
func pythonFingerprint(arduinoApp app.ArduinoApp) fingerprint {
	return partFingerprint(arduinoApp.ProvisioningStateDir(), "python")
}

// forget drops the record, so that no input matches it until the next save.
func (f fingerprint) forget() error {
	if err := f.path.Remove(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove the fingerprint: %w", err)
	}
	return nil
}

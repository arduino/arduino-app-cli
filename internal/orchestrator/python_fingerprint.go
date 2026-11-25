// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

// The Python part of an app is the compose project of its containers. What it is built from
// is the compose file rendered for this board, which names the images, the bricks, the
// ports and the environment of the project, and the model of each brick, which the brick
// reads from app.yaml and the compose file does not name. Its fingerprint is saved once
// compose brought that project up, so it tells whether the app folder still describes what
// runs.
func pythonFingerprint(arduinoApp app.ArduinoApp) fingerprint {
	return partFingerprint(arduinoApp.ProvisioningStateDir(), "python")
}

// pythonHashOf is the fingerprint hash of the Python part: the rendered compose file and,
// for each brick of app.yaml in its order, the model the brick runs with, which is the one
// the app names or the default of the brick.
func pythonHashOf(rendered []byte, bricks []app.Brick, index *bricksindex.BricksIndex) string {
	inputs := make([]string, 0, 1+2*len(bricks))
	inputs = append(inputs, string(rendered))
	for _, brick := range bricks {
		model := brick.Model
		if definition, found := index.FindBrickByID(brick.ID); found {
			model = cmp.Or(model, definition.ModelName)
		}
		inputs = append(inputs, brick.ID, model)
	}
	// Each input is quoted, so that none runs into the next.
	return hashOf(fmt.Appendf(nil, "%q", inputs))
}

// forget drops the record, so that no input matches it until the next save.
func (f fingerprint) forget() error {
	if err := f.path.Remove(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove the fingerprint: %w", err)
	}
	return nil
}

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import "github.com/arduino/arduino-app-cli/internal/orchestrator/app"

// pythonInputs is what the Python part is built from: the compose file rendered for this
// board, which names the images, the bricks, the ports and the environment of the project.
func pythonInputs(arduinoApp app.ArduinoApp) ([]byte, error) {
	return arduinoApp.AppComposeFilePath().ReadFile()
}

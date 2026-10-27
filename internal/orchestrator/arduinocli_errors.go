// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import "errors"

// ErrNoArduinoCli is what building or flashing a sketch answers where there is no
// arduino-cli, as in a browser (arduinocli_js.go).
var ErrNoArduinoCli = errors.New("this needs arduino-cli, which runs on a board: there is none here")

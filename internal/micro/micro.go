// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package micro

import (
	"time"
)

type GpioPin struct {
	Chip   string
	Number int
}

type Micro struct {
	resetPin GpioPin
}

func New(resetPin GpioPin) Micro {
	return Micro{
		resetPin: resetPin,
	}
}

func (m Micro) Reset() error {
	if err := m.Disable(); err != nil {
		return err
	}

	// Simulate a reset by toggling the reset pin
	time.Sleep(10 * time.Millisecond)

	return m.Enable()
}

func (m Micro) Enable() error {
	return enableOnBoard(m.resetPin.Chip, m.resetPin.Number)
}

func (m Micro) Disable() error {
	return disableOnBoard(m.resetPin.Chip, m.resetPin.Number)
}

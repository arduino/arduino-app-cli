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

const (
	ResetPin = 38
	ChipName = "gpiochip1"
)

func Reset() error {
	if err := Disable(); err != nil {
		return err
	}

	// Simulate a reset by toggling the reset pin
	time.Sleep(10 * time.Millisecond)

	return Enable()
}

func Enable() error {
	return enableOnBoard()
}

func Disable() error {
	return disableOnBoard()
}

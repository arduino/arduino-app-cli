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

//go:build linux
// +build linux

package micro

import (
	"github.com/warthog618/go-gpiocdev"
)

func enableOnBoard() error {
	chip, err := gpiocdev.NewChip(ChipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(ResetPin, gpiocdev.AsOutput(0))
	if err != nil {
		return err
	}
	defer line.Close()

	return line.SetValue(1)
}

func disableOnBoard() error {
	chip, err := gpiocdev.NewChip(ChipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(ResetPin, gpiocdev.AsOutput(0))
	if err != nil {
		return err
	}
	defer line.Close()

	return line.SetValue(0)
}

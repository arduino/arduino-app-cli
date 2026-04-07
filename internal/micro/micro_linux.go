// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

//go:build linux
// +build linux

package micro

import (
	"github.com/warthog618/go-gpiocdev"
)

func enableOnBoard(chipName string, resetPin int) error {
	chip, err := gpiocdev.NewChip(chipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(resetPin, gpiocdev.AsOutput(0))
	if err != nil {
		return err
	}
	defer line.Close()

	return line.SetValue(1)
}

func disableOnBoard(chipName string, resetPin int) error {
	chip, err := gpiocdev.NewChip(chipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(resetPin, gpiocdev.AsOutput(0))
	if err != nil {
		return err
	}
	defer line.Close()

	return line.SetValue(0)
}

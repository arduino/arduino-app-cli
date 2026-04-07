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

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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

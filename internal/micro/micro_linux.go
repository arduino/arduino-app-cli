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

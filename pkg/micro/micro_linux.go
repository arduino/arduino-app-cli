//go:build linux
// +build linux

package micro

import (
	"time"

	"github.com/warthog618/go-gpiocdev"
)

func resetOnBoard() error {
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

	if err := line.SetValue(0); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond) // Simulate reset delay
	if err := line.SetValue(1); err != nil {
		return err
	}

	return nil
}

func enableOnBoard(withReset bool) error {
	chip, err := gpiocdev.NewChip(ChipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(EnablePin, gpiocdev.AsOutput(0))
	if err != nil {
		return err
	}
	defer line.Close()

	if err := line.SetValue(0); err != nil {
		return err
	}

	if withReset {
		return resetOnBoard()
	}
	return nil
}

func disableOnBoard() error {
	chip, err := gpiocdev.NewChip(ChipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(EnablePin, gpiocdev.AsOutput(1))
	if err != nil {
		return err
	}
	defer line.Close()

	if err := line.SetValue(1); err != nil {
		return err
	}

	return resetOnBoard()
}

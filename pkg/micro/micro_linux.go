//go:build linux
// +build linux

package micro

import (
	"time"

	"github.com/warthog618/go-gpiocdev"
)

const (
	clipName  = "gpiochip1"
	resetPin  = 40
	enablePin = 71
)

func Reset() error {
	chip, err := gpiocdev.NewChip(clipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(resetPin, gpiocdev.AsOutput(0))
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

func Enable() error {
	chip, err := gpiocdev.NewChip(clipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(enablePin, gpiocdev.AsOutput(0))
	if err != nil {
		return err
	}
	defer line.Close()

	if err := line.SetValue(0); err != nil {
		return err
	}

	return Reset()
}

func Disable() error {
	chip, err := gpiocdev.NewChip(clipName)
	if err != nil {
		return err
	}
	defer chip.Close()

	line, err := chip.RequestLine(enablePin, gpiocdev.AsOutput(1))
	if err != nil {
		return err
	}
	defer line.Close()

	if err := line.SetValue(1); err != nil {
		return err
	}

	return Reset()
}

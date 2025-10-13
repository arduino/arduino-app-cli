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

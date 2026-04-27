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

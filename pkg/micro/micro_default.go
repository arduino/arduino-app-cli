//go:build !linux
// +build !linux

package micro

import "fmt"

func resetOnBoard() error {
	return fmt.Errorf("micro is not supported on this platform")
}

func enableOnBoard(bool) error {
	return fmt.Errorf("micro is not supported on this platform")
}

func disableOnBoard() error {
	return fmt.Errorf("Enable is not supported on this platform")
}

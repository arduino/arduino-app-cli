
//go:build !linux
// +build !linux

package micro

import "fmt"

func enableOnBoard(string, int) error {
	return fmt.Errorf("micro is not supported on this platform")
}

func disableOnBoard(string, int) error {
	return fmt.Errorf("Enable is not supported on this platform")
}

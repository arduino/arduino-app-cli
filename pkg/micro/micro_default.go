//go:build !linux
// +build !linux

package micro

import "fmt"

func Reset() error {
	return fmt.Errorf("micro is not supported on this platform")
}

func Enable() error {
	return fmt.Errorf("micro is not supported on this platform")
}

func Disable() error {
	return fmt.Errorf("Enable is not supported on this platform")
}

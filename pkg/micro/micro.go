package micro

import (
	"context"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

const (
	ResetPin = 38
	ChipName = "gpiochip1"
)

var OnBoard = sync.OnceValue(func() bool {
	var boardNames = []string{"UNO Q\n", "Imola\n", "Inc. Robotics RB1\n"}
	buf, err := os.ReadFile("/sys/class/dmi/id/product_name")
	if err == nil && slices.Contains(boardNames, string(buf)) {
		return true
	}
	return false
})()

func Reset(ctx context.Context, cmder remote.RemoteShell) error {
	if err := Disable(ctx, cmder); err != nil {
		return err
	}

	// Simulate a reset by toggling the reset pin
	time.Sleep(10 * time.Millisecond)

	return Enable(ctx, cmder)
}

func Enable(ctx context.Context, cmder remote.RemoteShell) error {
	if OnBoard {
		return enableOnBoard()
	}

	return enableRemote(ctx, cmder)
}

func Disable(ctx context.Context, cmder remote.RemoteShell) error {
	if OnBoard {
		return disableOnBoard()
	}

	return disableRemote(ctx, cmder)
}

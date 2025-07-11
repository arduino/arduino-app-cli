package micro

import (
	"context"
	"os"
	"slices"
	"sync"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

const (
	ResetPin  = 38
	EnablePin = 70
	ChipName  = "gpiochip1"
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
	if OnBoard {
		return resetOnBoard()
	}

	return resetRemote(ctx, cmder)
}

func Enable(ctx context.Context, cmder remote.RemoteShell, withReset bool) error {
	if OnBoard {
		return enableOnBoard(withReset)
	}

	return enableRemote(ctx, cmder, withReset)
}

func Disable(ctx context.Context, cmder remote.RemoteShell) error {
	if OnBoard {
		return disableOnBoard()
	}

	return disableRemote(ctx, cmder)
}

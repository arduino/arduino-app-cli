package micro

import (
	"context"
	"fmt"
	"time"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

var chipDev = fmt.Sprintf("/dev/%s", ChipName)

func resetRemote(ctx context.Context, cmder remote.RemoteShell) error {
	if err := cmder.GetCmd(ctx, "gpioset", "-c", chipDev, "-t0", fmt.Sprintf("%d=0", ResetPin)).Run(); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond) // Simulate reset delay
	if err := cmder.GetCmd(ctx, "gpioset", "-c", chipDev, "-t0", fmt.Sprintf("%d=1", ResetPin)).Run(); err != nil {
		return err
	}
	return nil
}

func enableRemote(ctx context.Context, cmder remote.RemoteShell, withReset bool) error {
	if err := cmder.GetCmd(context.Background(), "gpioset", "-c", chipDev, "-t0", fmt.Sprintf("%d=0", EnablePin)).Run(); err != nil {
		return err
	}
	if withReset {
		return resetRemote(ctx, cmder)
	}
	return nil
}

func disableRemote(ctx context.Context, cmder remote.RemoteShell) error {
	if err := cmder.GetCmd(ctx, "gpioset", "-c", chipDev, "-t0", fmt.Sprintf("%d=1", EnablePin)).Run(); err != nil {
		return err
	}

	return resetRemote(ctx, cmder)
}

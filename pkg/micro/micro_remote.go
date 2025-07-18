package micro

import (
	"context"
	"fmt"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

var chipDev = fmt.Sprintf("/dev/%s", ChipName)

func enableRemote(ctx context.Context, cmder remote.RemoteShell) error {
	return cmder.GetCmd(ctx, "gpioset", "-c", chipDev, "-t0", fmt.Sprintf("%d=1", ResetPin)).
		Run()
}

func disableRemote(ctx context.Context, cmder remote.RemoteShell) error {
	return cmder.GetCmd(ctx, "gpioset", "-c", chipDev, "-t0", fmt.Sprintf("%d=0", ResetPin)).
		Run()
}

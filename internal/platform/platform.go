package platform

import (
	"log/slog"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/micro"
	"github.com/arduino/arduino-app-cli/pkg/x/devicetree"
)

type GpioPin struct {
	Chip   string
	Number int
}

type Platform struct {
	FQBN        string
	PlatformID  string
	CompileJobs int32
	Linux       struct {
		UserLeds   paths.PathList
		StatusLeds paths.PathList
	}
	Micro struct {
		ResetPin GpioPin
	}
}

func GetPlatform() Platform {
	compatible := devicetree.LoadCompatible()
	slog.Debug("detected platform", "compatible", compatible)
	switch {
	case compatible.IsCompatibleWith("arduino,imola"):
		return Platform{
			FQBN:       "arduino:zephyr:unoq",
			PlatformID: "arduino:zephyr",
			Linux: struct{ UserLeds, StatusLeds paths.PathList }{
				StatusLeds: paths.NewPathList(
					"/sys/class/leds/blue:bt",
					"/sys/class/leds/green:wlan",
					"/sys/class/leds/red:panic",
				),
				UserLeds: paths.NewPathList(
					"/sys/class/leds/blue:user",
					"/sys/class/leds/green:user",
					"/sys/class/leds/red:user",
				),
			},
			CompileJobs: 2,
			Micro: struct{ ResetPin GpioPin }{
				ResetPin: GpioPin{Chip: "gpiochip1", Number: 38},
			},
		}
	case compatible.IsCompatibleWith("arduino,monza"):
		return Platform{
			FQBN:       "arduino:zephyr:ventunoq",
			PlatformID: "arduino:zephyr",
			Linux: struct{ UserLeds, StatusLeds paths.PathList }{
				// TODO: add leds paths
				StatusLeds: paths.NewPathList(),
				UserLeds:   paths.NewPathList(),
			},
			CompileJobs: 0, // unlimited
			Micro: struct{ ResetPin GpioPin }{
				ResetPin: GpioPin{Chip: "gpiochip2", Number: 78},
			},
		}
	default:
		slog.Warn("not supported platform", "compatible", compatible)
		return Platform{}
	}
}

func (p Platform) GetMicro() micro.Micro {
	return micro.New(micro.GpioPin(p.Micro.ResetPin))
}

func (p Platform) SupportFlashToRam() bool {
	return p.FQBN == "arduino:zephyr:unoq"
}

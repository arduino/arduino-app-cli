package platform

import (
	"os"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/micro"
)

type GpioPin struct {
	Chip   string
	Number int
}

type Platform struct {
	BoardName    string
	FQBN         string
	PlatformName string
	Linux        struct {
		UserLeds   paths.PathList
		StatusLeds paths.PathList
	}
	Micro struct {
		ResetPin GpioPin
	}
}

func GetPlatform() Platform {
	boardName := getBoardName()
	switch boardName {
	case "Imola":
		return Platform{
			BoardName:    "Imola",
			FQBN:         "arduino:zephyr:unoq",
			PlatformName: "arduino:zephyr",
			Linux: struct{ UserLeds, StatusLeds paths.PathList }{
				StatusLeds: paths.NewPathList(
					"/sys/class/leds/blue:bt/trigger",
					"/sys/class/leds/green:wlan/trigger",
					"/sys/class/leds/red:panic/trigger",
				),
				UserLeds: paths.NewPathList(
					"/sys/class/leds/blue:user",
					"/sys/class/leds/green:user",
					"/sys/class/leds/red:user",
				),
			},
			Micro: struct{ ResetPin GpioPin }{
				ResetPin: GpioPin{Chip: "gpiochip0", Number: 6},
			},
		}
	default:
		panic("unsupported board: " + boardName)
	}
}

func (p Platform) GetMicro() micro.Micro {
	return micro.New(micro.GpioPin(p.Micro.ResetPin))
}

func getBoardName() string {
	if buf, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
		return strings.TrimSpace(string(buf))
	} else if buf, err := os.ReadFile("/sys/firmware/devicetree/base/model"); err == nil {
		idx := strings.LastIndex(string(buf), ",")
		if idx != -1 {
			return strings.TrimSpace(string(buf[:idx]))
		}
		if idx := strings.LastIndex(string(buf), " "); idx != -1 {
			return strings.TrimSpace(string(buf[:idx]))
		}
	}
	return ""
}

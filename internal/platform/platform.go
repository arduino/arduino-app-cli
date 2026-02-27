package platform

import (
	"bytes"
	"log/slog"
	"os"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/micro"
)

type GpioPin struct {
	Chip   string
	Number int
}

type Platform struct {
	Name       string
	FQBN       string
	PlatformID string
	Linux      struct {
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
			Name:       "Imola",
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
			Micro: struct{ ResetPin GpioPin }{
				ResetPin: GpioPin{Chip: "gpiochip1", Number: 38},
			},
		}
	default:
		slog.Warn("not supported platform", "boardName", boardName)
		return Platform{
			Name: boardName,
		}
	}
}

func (p Platform) GetMicro() micro.Micro {
	return micro.New(micro.GpioPin(p.Micro.ResetPin))
}

func getBoardName() string {
	trimAll := func(s []byte) []byte {
		return bytes.Trim(s, " \n\t\r\x00")
	}

	if buf, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
		return string(trimAll(buf))
	} else if buf, err := os.ReadFile("/sys/firmware/devicetree/base/model"); err == nil {
		idx := bytes.LastIndex(buf, []byte(","))
		if idx != -1 {
			return string(trimAll(buf[:idx]))
		}
		if idx := bytes.LastIndex(buf, []byte(" ")); idx != -1 {
			return string(trimAll(buf[idx+1:]))
		}
	}

	return ""
}

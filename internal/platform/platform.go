package platform

import (
	"bytes"
	"io"
	"io/fs"
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
	codeName   string
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
	codeName := getCodeName()
	switch codeName {
	case "Imola":
		slog.Debug("detected platform", "codeName", codeName)
		return Platform{
			codeName:   codeName,
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
		slog.Warn("not supported platform", "codeName", codeName)
		return Platform{
			codeName: codeName,
		}
	}
}

func (p Platform) GetMicro() micro.Micro {
	return micro.New(micro.GpioPin(p.Micro.ResetPin))
}

func getCodeName() string {
	return getCodeNameInternal(os.DirFS("/"))
}

func getCodeNameInternal(fs fs.FS) string {
	trimAll := func(s []byte) []byte {
		return bytes.Trim(s, " \n\t\r\x00")
	}

	readFile := func(path string) ([]byte, error) {
		f, err := fs.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if buf, err := io.ReadAll(f); err != nil {
			return nil, err
		} else {
			return buf, nil
		}
	}

	if buf, err := readFile("sys/class/dmi/id/product_name"); err == nil {
		return string(trimAll(buf))
	} else if buf, err := readFile("sys/firmware/devicetree/base/model"); err == nil {
		if idx := bytes.LastIndex(buf, []byte(",")); idx != -1 {
			return string(trimAll(buf[idx+1:]))
		}
		if idx := bytes.LastIndex(buf, []byte(" ")); idx != -1 {
			return string(trimAll(buf[idx+1:]))
		}
	}

	return ""
}

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
	case "imola":
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
	trimAndLower := func(s []byte) []byte {
		return bytes.ToLower(bytes.Trim(s, " \n\t\r\x00"))
	}

	readFile := func(path string) ([]byte, error) {
		f, err := fs.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(f)
	}

	if buf, err := readFile("sys/class/dmi/id/product_name"); err == nil {
		return string(trimAndLower(buf))
	} else if buf, err := readFile("sys/firmware/devicetree/base/compatible"); err == nil {
		compatibles := bytes.Split(buf, []byte{'\x00'})
		if len(compatibles) > 0 {
			compatible := compatibles[0]
			if idx := bytes.Index(compatibles[0], []byte{','}); idx != -1 {
				return string(trimAndLower(compatible[idx+1:]))
			} else {
				return string(trimAndLower(compatible))
			}
		}
	}

	return ""
}

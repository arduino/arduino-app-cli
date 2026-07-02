// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package platform

import (
	"encoding/json"
	"fmt"
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
	BoardName   string `json:"board_name"`
	FQBN        string `json:"fqbn"`
	PlatformID  string `json:"-"`
	CompileJobs int32  `json:"-"`
	Linux       struct {
		BoardLeds paths.PathList
	} `json:"-"`
	Micro struct {
		ResetPin GpioPin
	} `json:"-"`
}

func GetPlatform(dir *paths.Path) Platform {
	compatible := devicetree.LoadCompatible()
	slog.Debug("detected platform", "compatible", compatible)
	var platform Platform
	switch {
	case compatible.IsCompatibleWith("arduino,imola"):
		platform = Platform{
			FQBN:       "arduino:zephyr:unoq",
			PlatformID: "arduino:zephyr",
			BoardName:  "unoq",
			Linux: struct{ BoardLeds paths.PathList }{
				BoardLeds: GetUnoQBoardLeds(),
			},
			CompileJobs: 2,
			Micro: struct{ ResetPin GpioPin }{
				ResetPin: GpioPin{Chip: "gpiochip1", Number: 38},
			},
		}
	case compatible.IsCompatibleWith("arduino,monza"):
		platform = Platform{
			FQBN:       "arduino:zephyr:ventunoq",
			PlatformID: "arduino:zephyr",
			BoardName:  "ventunoq",
			Linux: struct{ BoardLeds paths.PathList }{
				BoardLeds: GetVentunoQBoardLeds(),
			},
			CompileJobs: 0, // unlimited
			Micro: struct{ ResetPin GpioPin }{
				ResetPin: GpioPin{Chip: "gpiochip2", Number: 78},
			},
		}
	default:
		slog.Warn("not supported platform", "compatible", compatible)
	}

	if dir != nil {
		if filePath := dir.Join("platform.json"); filePath.Exist() {
			if f, err := filePath.Open(); err == nil {
				defer f.Close()
				if err = json.NewDecoder(f).Decode(&platform); err == nil {
					slog.Debug("loaded override from platform.json file", "file", filePath.String(), "platform", platform)
				} else {
					slog.Warn("failed to decode override platform.json file", "file", filePath.String(), "error", err)
				}
			} else {
				slog.Warn("failed to open override platform.json file", "file", filePath.String(), "error", err)
			}
		}
	}

	slog.Info("using platform config", "platform", platform)
	return platform
}

func (p Platform) GetMicro() micro.Micro {
	return micro.New(micro.GpioPin(p.Micro.ResetPin))
}

func (p Platform) SupportFlashToRam() bool {
	return p.FQBN == "arduino:zephyr:unoq"
}

type EIDeploymentParams struct {
	ModelType  string
	Engine     string
	DeviceType string
}

func (p Platform) EIDeploymentParams() (EIDeploymentParams, error) {
	switch p.BoardName {
	case "unoq":
		return EIDeploymentParams{ModelType: "float32", Engine: "tflite", DeviceType: "runner-linux-aarch64"}, nil
	case "ventunoq":
		return EIDeploymentParams{ModelType: "float32", Engine: "tflite", DeviceType: "runner-linux-aarch64-qnn"}, nil
	default:
		return EIDeploymentParams{}, fmt.Errorf("unsupported platform %q for Edge Impulse deployment", p.BoardName)
	}
}

func GetUnoQBoardLeds() paths.PathList {
	// new leds paths
	newPaths := paths.NewPathList(
		// LED 1
		"/dev/leds/builtin/led1_b",
		"/dev/leds/builtin/led1_g",
		"/dev/leds/builtin/led1_r",
		// LED 2
		"/dev/leds/builtin/led2_b",
		"/dev/leds/builtin/led2_g",
		"/dev/leds/builtin/led2_r",
	)

	// legacy paths, old code expects this ones
	legacyPaths := paths.NewPathList(
		// LED 1
		"/sys/class/leds/blue:user",
		"/sys/class/leds/green:user",
		"/sys/class/leds/red:user",
		// LED 2
		"/sys/class/leds/blue:bt",
		"/sys/class/leds/green:wlan",
		"/sys/class/leds/red:panic",
	)

	if newPaths[0].Exist() {
		return newPaths
	}
	return legacyPaths
}

func GetVentunoQBoardLeds() paths.PathList {
	return paths.NewPathList(
		// LED 1
		"/dev/leds/builtin/led1_b",
		"/dev/leds/builtin/led1_g",
		"/dev/leds/builtin/led1_r",
		// LED 2
		"/dev/leds/builtin/led2_b",
		"/dev/leds/builtin/led2_g",
		"/dev/leds/builtin/led2_r",
		// LED 3
		"/dev/leds/builtin/led3_b",
		"/dev/leds/builtin/led3_g",
		"/dev/leds/builtin/led3_r",
		// LED 4
		"/dev/leds/builtin/led4_b",
		"/dev/leds/builtin/led4_g",
		"/dev/leds/builtin/led4_r",
	)
}

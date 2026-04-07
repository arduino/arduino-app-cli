// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"slices"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/sirupsen/logrus"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

func legacyUploadSketchInRam(ctx context.Context,
	w io.Writer,
	srv rpc.ArduinoCoreServiceServer,
	inst *rpc.Instance,
	platform platform.Platform,
	sketchPath string,
	buildPath string,
) error {
	upload := func() error {
		stream, _ := commands.UploadToServerStreams(ctx, w, w)
		if err := srv.Upload(&rpc.UploadRequest{
			Instance:   inst,
			Fqbn:       platform.FQBN + ":flash_mode=ram",
			SketchPath: sketchPath,
			ImportDir:  buildPath,
		}, stream); err != nil {
			return err
		}
		return nil
	}
	if err := upload(); err != nil {
		slog.Warn("failed to upload in ram mode, trying to configure the board in ram mode, and retry", slog.String("error", err.Error()))
		if err := configureMicroInRamMode(ctx, w, srv, inst, platform); err != nil {
			return err
		}
	}
	return upload()
}

// configureMicroInRamMode uploads an empty binary overing any sketch previously uploaded in flash.
// This is required to be able to upload sketches in ram mode after if there is already a sketch in flash.
func configureMicroInRamMode(
	ctx context.Context,
	w io.Writer,
	srv rpc.ArduinoCoreServiceServer,
	inst *rpc.Instance,
	platform platform.Platform,
) error {
	emptyBinDir := paths.New("/tmp/empty")
	_ = emptyBinDir.MkdirAll()
	defer func() { _ = emptyBinDir.RemoveAll() }()

	zeros, err := os.Open("/dev/zero")
	if err != nil {
		return err
	}
	defer zeros.Close()

	empty, err := emptyBinDir.Join("empty.ino.elf-zsk.bin").Create()
	if err != nil {
		return err
	}
	defer empty.Close()
	if _, err := io.CopyN(empty, zeros, 50); err != nil {
		return err
	}

	stream, _ := commands.UploadToServerStreams(ctx, w, w)
	return srv.Upload(&rpc.UploadRequest{
		Instance:  inst,
		Fqbn:      platform.FQBN + ":flash_mode=flash",
		ImportDir: emptyBinDir.String(),
	}, stream)
}

type MenuOptions []MenuOption

type MenuOption struct {
	name   string
	values []string
}

var WaitForApp = MenuOptionValue{name: "wait_linux_boot", value: "app"}

type MenuOptionValue struct {
	name  string
	value string
}

func (o MenuOptionValue) String() string {
	return o.name + "=" + o.value
}

func (o MenuOptions) Has(optionValue MenuOptionValue) bool {
	return slices.ContainsFunc(o, func(option MenuOption) bool {
		if option.name == optionValue.name {
			return slices.Contains(option.values, optionValue.value)
		}
		return false
	})
}

func GetPlatformMenuOptions(ctx context.Context, platform platform.Platform) (MenuOptions, error) {
	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if _, err := srv.SettingsSetValue(ctx, &rpc.SettingsSetValueRequest{
		Key:          "network.connection_timeout",
		EncodedValue: "600s",
		ValueFormat:  "cli",
	}); err != nil {
		return MenuOptions{}, err
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return MenuOptions{}, err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	if err := srv.Init(
		&rpc.InitRequest{Instance: inst},
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			slog.Debug("Arduino init instance", slog.String("instance", r.String()))
			return nil
		}),
	); err != nil {
		return MenuOptions{}, err
	}

	info, err := srv.BoardDetails(ctx, &rpc.BoardDetailsRequest{
		Instance: inst,
		Fqbn:     platform.FQBN,
	})
	if err != nil {
		return MenuOptions{}, err
	}

	var options MenuOptions
	for _, config := range info.GetConfigOptions() {
		option := MenuOption{name: config.GetOption()}
		for _, value := range config.GetValues() {
			option.values = append(option.values, value.GetValue())
		}
		options = append(options, option)
	}
	return options, nil
}

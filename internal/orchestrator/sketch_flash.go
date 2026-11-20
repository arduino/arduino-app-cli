// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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

	"github.com/arduino/arduino-app-cli/internal/platform"
)

func legacyUploadSketchInRam(ctx context.Context,
	w io.Writer,
	srv rpc.ArduinoCoreServiceServer,
	inst *rpc.Instance,
	fqbn string,
	sketchPath string,
	buildPath string,
) error {
	upload := func() error {
		stream, _ := commands.UploadToServerStreams(ctx, w, w)
		if err := srv.Upload(&rpc.UploadRequest{
			Instance:   inst,
			Fqbn:       fqbn + ":flash_mode=ram",
			SketchPath: sketchPath,
			ImportDir:  buildPath,
		}, stream); err != nil {
			return err
		}
		return nil
	}
	if err := upload(); err != nil {
		slog.Warn("failed to upload in ram mode, trying to configure the board in ram mode, and retry", slog.String("error", err.Error()))
		if err := configureMicroInRamMode(ctx, w, srv, inst, fqbn); err != nil {
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
	fqbn string,
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
		Fqbn:      fqbn + ":flash_mode=flash",
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
	info, err := boardDetails(ctx, platform)
	if err != nil {
		return MenuOptions{}, err
	}
	return menuOptionsOf(info), nil
}

// boardDetails asks arduino-cli about the board of the platform: what the installed platform
// says of it, with or without a sketch profile loaded.
func boardDetails(ctx context.Context, platform platform.Platform) (*rpc.BoardDetailsResponse, error) {
	var info *rpc.BoardDetailsResponse
	err := cli.forPlatform(ctx, func(srv rpc.ArduinoCoreServiceServer, inst *rpc.Instance) error {
		var err error
		info, err = srv.BoardDetails(ctx, &rpc.BoardDetailsRequest{Instance: inst, Fqbn: platform.FQBN})
		return err
	})
	return info, err
}

// menuOptionsOf are the menu options of the board, none for a board arduino-cli did not describe.
func menuOptionsOf(info *rpc.BoardDetailsResponse) MenuOptions {
	options := make(MenuOptions, 0, len(info.GetConfigOptions()))
	for _, config := range info.GetConfigOptions() {
		option := MenuOption{name: config.GetOption()}
		for _, value := range config.GetValues() {
			option.values = append(option.values, value.GetValue())
		}
		options = append(options, option)
	}
	return options
}

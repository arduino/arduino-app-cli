package board

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/arduino/arduino-cli/commands"
	"github.com/arduino/arduino-cli/pkg/fqbn"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/sirupsen/logrus"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
)

type Board struct {
	// TODO: add fields for allowing identification
	Protocol string
	Serial   string
	Address  string
}

const (
	SerialProtocol  = "serial"
	NetworkProtocol = "network"
)

const (
	ArduinoUnoQ = "arduino:zephyr:unoq"
)

func FromFQBN(ctx context.Context, fqbn string) ([]Board, error) {
	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return nil, err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	if err := srv.Init(
		&rpc.InitRequest{Instance: inst},
		// TODO: implement progress callback function
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error { return nil }),
	); err != nil {
		return nil, err
	}

	list, err := srv.BoardList(ctx, &rpc.BoardListRequest{
		Instance: inst,
		Timeout:  2000, // 2 seconds
		Fqbn:     fqbn,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get info for FQBN %s: %w", fqbn, err)
	}

	if ports := list.GetPorts(); len(ports) != 0 {
		var boards []Board
		for _, port := range ports {
			if port.GetPort() == nil {
				continue
			}
			switch port.GetPort().GetProtocol() {
			case SerialProtocol:
				serial := strings.ToLower(port.GetPort().GetHardwareId()) // in windows this is uppercase.
				boards = append(boards, Board{
					Protocol: SerialProtocol,
					Serial:   serial,
				})
			case NetworkProtocol:
				boards = append(boards, Board{
					Protocol: NetworkProtocol,
					Address:  port.GetPort().GetAddress(),
				})
			default:
				slog.Warn("unknown protocol", "protocol", port.GetPort().GetProtocol())
			}
		}

		// Sort serial first
		slices.SortFunc(boards, func(a, b Board) int {
			if a.Protocol == "serial" {
				return -1
			} else {
				return 1
			}
		})

		return boards, nil
	}

	return nil, fmt.Errorf("no hardware ID found for FQBN %s", fqbn)
}

func (b *Board) Connect() (remote.RemoteConn, error) {
	switch b.Protocol {
	case SerialProtocol:
		return adb.FromSerial(b.Serial, "")
	case NetworkProtocol:
		// TODO: use secure connection
		return ssh.FromHost("arduino", "arduino", b.Address+":22")
	default:
		panic("unreachable")
	}
}

func EnsurePlatformInstalled(ctx context.Context, rawFQBN string) error {
	parsedFQBN, err := fqbn.Parse(rawFQBN)
	if err != nil {
		return err
	}

	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, err := srv.CleanDownloadCacheDirectory(ctx, &rpc.CleanDownloadCacheDirectoryRequest{})
		if err != nil {
			slog.Error("Error cleaning cache directory", slog.Any("error", err))
		}
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	// TODO: after embargo remove this
	_, err = srv.SettingsSetValue(ctx, &rpc.SettingsSetValueRequest{
		Key:          "board_manager.additional_urls",
		EncodedValue: "https://arduino:aptexperiment@apt-repo.oniudra.cc/zephyr-core-imola.json",
		ValueFormat:  "cli",
	})
	if err != nil {
		return err
	}

	stream, _ := commands.UpdateIndexStreamResponseToCallbackFunction(ctx, func(curr *rpc.DownloadProgress) {
		slog.Debug("Update index progress", slog.String("download_progress", curr.String()))
	})
	if err := srv.UpdateIndex(&rpc.UpdateIndexRequest{Instance: inst}, stream); err != nil {
		return err
	}

	if err := srv.Init(
		&rpc.InitRequest{Instance: inst},
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			slog.Debug("Arduino init instance", slog.String("instance", r.String()))
			return nil
		}),
	); err != nil {
		return err
	}

	platforms, err := srv.PlatformSearch(ctx, &rpc.PlatformSearchRequest{
		Instance:          inst,
		ManuallyInstalled: true,
	})
	if err != nil {
		return err
	}

	var platformSummary *rpc.PlatformSummary
	for _, v := range platforms.GetSearchOutput() {
		if v.GetMetadata().GetId() == parsedFQBN.Vendor+":"+parsedFQBN.Architecture {
			platformSummary = v
			break
		}
	}
	if platformSummary == nil {
		return fmt.Errorf("platform %s not found", parsedFQBN.Vendor+":"+parsedFQBN.Architecture)
	}

	if platformSummary.GetInstalledVersion() != "" {
		return nil
	}

	return srv.PlatformInstall(
		&rpc.PlatformInstallRequest{
			Instance:        inst,
			PlatformPackage: parsedFQBN.Vendor,
			Architecture:    parsedFQBN.Architecture,
		},
		commands.PlatformInstallStreamResponseToCallbackFunction(
			ctx,
			func(curr *rpc.DownloadProgress) {
				slog.Debug("Platform install progress", slog.String("download_progress", curr.String()))
			},
			func(msg *rpc.TaskProgress) {
				slog.Debug("Platform install message", slog.String("message", msg.GetMessage()))
			},
		),
	)
}

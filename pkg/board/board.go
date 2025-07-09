package board

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/arduino/arduino-cli/commands"
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
				boards = append(boards, Board{
					Protocol: SerialProtocol,
					Serial:   port.GetPort().GetHardwareId(),
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

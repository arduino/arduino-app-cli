// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package board

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/arduino/arduino-cli/commands"
	"github.com/arduino/arduino-cli/pkg/fqbn"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/sirupsen/logrus"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/local"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
)

type Board struct {
	Protocol   string
	Serial     string
	Address    string
	CustomName string
	BoardName  string
}

const (
	SerialProtocol  = "serial"
	NetworkProtocol = "network"
	LocalProtocol   = "local"
)

const (
	ArduinoUnoQ = "arduino:zephyr:unoq"
	SerialPath  = "/sys/devices/soc0/serial_number"
)

func identifyUnoQ(p *rpc.DetectedPort) {
	const UnoQVID = "0x2341"
	const UnoQPID = "0x0078"
	const UnoQBoardID = "unoq"

	// If the board has been already identified as Uno Q, just return true
	for _, b := range p.GetMatchingBoards() {
		if b.GetFqbn() == ArduinoUnoQ {
			return
		}
	}

	// Otherwise check the VID/PID or board ID
	props := p.GetPort().GetProperties()
	isUnoQ := props["board"] == UnoQBoardID || (props["vid"] == UnoQVID && props["pid"] == UnoQPID)
	if isUnoQ {
		p.MatchingBoards = append(p.MatchingBoards, &rpc.BoardListItem{Name: "Arduino UNO Q", Fqbn: ArduinoUnoQ})
	}
}

var onBoard = sync.OnceValue(func() bool {
	var boardNames = []string{"UNO Q\n", "Imola\n", "Inc. Robotics RB1\n"}
	buf, err := os.ReadFile("/sys/class/dmi/id/product_name")
	if err == nil && slices.Contains(boardNames, string(buf)) {
		return true
	}
	return false
})()

// Cache the initialized Arduino CLI service, so it don't need to be re-initialized
// TODO: provide a way to get the board information by event instead of polling.
var arduinoCLIServer rpc.ArduinoCoreServiceServer
var arduinoCLIInstance *rpc.Instance
var arduinoCLILock sync.Mutex

func FromFQBN(ctx context.Context, fqbn string) ([]Board, error) {
	arduinoCLILock.Lock()
	defer arduinoCLILock.Unlock()

	if onBoard {
		var customName string
		if name, err := GetCustomName(ctx, &local.LocalConnection{}); err == nil {
			customName = name
		}
		var serial string
		if sn, err := getSerial(&local.LocalConnection{}); err == nil {
			serial = sn
		}
		return []Board{{
			Protocol:   LocalProtocol,
			Serial:     serial,
			Address:    "",
			CustomName: customName,
			BoardName:  "Uno Q",
		}}, nil
	}

	if arduinoCLIServer == nil {
		logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
		arduinoCLIServer = commands.NewArduinoCoreServer()
	}

	if arduinoCLIInstance == nil {
		var inst *rpc.Instance
		if resp, err := arduinoCLIServer.Create(ctx, &rpc.CreateRequest{}); err != nil {
			return nil, err
		} else {
			inst = resp.GetInstance()
		}

		if err := arduinoCLIServer.Init(
			&rpc.InitRequest{Instance: inst},
			// TODO: implement progress callback function
			commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error { return nil }),
		); err != nil {
			// in case of error destroy invalid instance
			_, _ = arduinoCLIServer.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
			return nil, err
		}

		arduinoCLIInstance = inst
	}

	listReq := &rpc.BoardListRequest{
		Instance: arduinoCLIInstance,
		Timeout:  100, // 100 ms
	}
	list, err := arduinoCLIServer.BoardList(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get info for FQBN %s: %w", fqbn, err)
	}

	ports := list.GetPorts()
	for _, p := range ports {
		identifyUnoQ(p)
	}

	portMatchFqbn := func(p *rpc.DetectedPort) bool {
		return slices.ContainsFunc(
			p.GetMatchingBoards(),
			func(b *rpc.BoardListItem) bool {
				return b.GetFqbn() == fqbn
			},
		)
	}
	ports = f.Filter(ports, portMatchFqbn)

	if len(ports) == 0 {
		return nil, fmt.Errorf("no hardware ID found for FQBN %s", fqbn)
	}

	var boards []Board
	for _, port := range ports {
		if port.GetPort() == nil {
			continue
		}

		var boardName string
		if len(port.GetMatchingBoards()) > 0 {
			boardName = port.GetMatchingBoards()[0].GetName()
		}

		switch port.GetPort().GetProtocol() {
		case SerialProtocol:
			serial := strings.ToLower(port.GetPort().GetHardwareId()) // in windows this is uppercase.
			// TODO: we should store the board custom name in the product id so we can get it from the discovery service.
			var customName string
			if conn, err := adb.FromSerial(serial, ""); err == nil {
				if name, err := GetCustomName(ctx, conn); err == nil {
					customName = name
				}
			}

			boards = append(boards, Board{
				Protocol:   SerialProtocol,
				Serial:     serial,
				BoardName:  boardName,
				CustomName: customName,
			})
		case NetworkProtocol:
			var customName string
			if name, ok := port.GetPort().GetProperties()["hostname"]; ok {
				// take the part before the first dot as custom name
				idx := strings.Index(name, ".")
				if idx == -1 {
					idx = len(name)
				}
				customName = name[:idx]
			}
			var serial string
			if sn, ok := port.GetPort().GetProperties()["serial_number"]; ok {
				serial = sn
			}

			boards = append(boards, Board{
				Protocol:   NetworkProtocol,
				Address:    port.GetPort().GetAddress(),
				Serial:     serial,
				BoardName:  boardName,
				CustomName: customName,
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

func (b *Board) GetConnection(optPassword ...string) (remote.RemoteConn, error) {
	if len(optPassword) > 1 {
		return nil, fmt.Errorf("too many optional args, expected at most one")
	}

	password := "arduino"
	if len(optPassword) == 1 {
		password = optPassword[0]
	}

	switch b.Protocol {
	case SerialProtocol:
		return adb.FromSerial(b.Serial, "")
	case NetworkProtocol:
		return ssh.FromHost("arduino", password, net.JoinHostPort(b.Address, "22"))
	case LocalProtocol:
		return &local.LocalConnection{}, nil
	default:
		panic("unreachable")
	}
}

var customNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

func SetCustomName(ctx context.Context, conn remote.RemoteConn, name string) error {
	if !customNameRegex.MatchString(name) {
		return fmt.Errorf("invalid custom name: %s, must match regex %s", name, customNameRegex.String())
	}

	err := conn.GetCmd("sudo", "hostnamectl", "set-hostname", name).
		Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to set board name: %w", err)

	}

	isEnable, err := NetworkModeStatus(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed get board status: %w", err)
	}
	if isEnable {
		cmds := [][]string{
			{"sudo", "systemctl", "stop", "avahi-daemon"},
			{"sudo", "systemctl", "start", "avahi-daemon"},
		}
		for _, cmd := range cmds {
			if out, err := conn.GetCmd(cmd[0], cmd[1:]...).Output(ctx); err != nil {
				return fmt.Errorf("failed to run cmd %q: %w: %s", strings.Join(cmd, " "), err, string(out))
			}
		}
	}

	return nil
}

func GetCustomName(ctx context.Context, conn remote.RemoteConn) (string, error) {
	r, err := conn.ReadFile("/etc/hostname")
	if err != nil {
		return "", fmt.Errorf("failed to get board name: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read board name: %w", err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func IsUserPasswordSet(conn remote.RemoteShell) (bool, error) {
	cmd := conn.GetCmd("chage", "-l", "arduino")

	w, out, _, closer, err := cmd.Interactive()
	if err != nil {
		return false, fmt.Errorf("failed to check password: %w", err)
	}
	w.Close() // we don't need to write anything

	isUserSet, err := remote.ParseChage(out)
	if err != nil {
		return false, err
	}
	if err := closer(); err != nil {
		return false, err
	}
	return isUserSet, nil
}

func SetUserPassword(ctx context.Context, conn remote.RemoteConn, newPass string) error {
	cmd := conn.GetCmd("sudo", "arduino-passwd")
	stdin, stdout, stderr, closer, err := cmd.Interactive()
	if err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	if _, err = stdin.Write([]byte(newPass)); err != nil {
		return fmt.Errorf("failed to write password: %w", err)
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	if err := closer(); err != nil {
		out, _ := io.ReadAll(stdout)
		errOut, _ := io.ReadAll(stderr)
		return fmt.Errorf("failed to set password: %w: %s %s", err, out, errOut)
	}

	return nil
}

func EnableNetworkMode(ctx context.Context, conn remote.RemoteConn) error {
	cmds := [][]string{
		{"sudo", "dpkg-reconfigure", "openssh-server"},
		{"sudo", "systemctl", "enable", "ssh"},
		{"sudo", "systemctl", "start", "ssh"},
		{"sudo", "systemctl", "enable", "avahi-daemon"},
		{"sudo", "systemctl", "start", "avahi-daemon"},
	}

	for _, cmd := range cmds {
		if out, err := conn.GetCmd(cmd[0], cmd[1:]...).Output(ctx); err != nil {
			return fmt.Errorf("failed to run cmd %q: %w: %s", strings.Join(cmd, " "), err, string(out))
		}
	}

	return nil
}

func NetworkModeStatus(ctx context.Context, conn remote.RemoteConn) (bool, error) {
	cmds := [][]string{
		{"systemctl", "is-enabled", "ssh"},
		{"systemctl", "is-active", "ssh"},
		{"systemctl", "is-enabled", "avahi-daemon"},
		{"systemctl", "is-active", "avahi-daemon"},
	}

	for _, cmd := range cmds {
		if err := conn.GetCmd(cmd[0], cmd[1:]...).Run(ctx); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() != 0 {
				return false, nil
			}
			return false, fmt.Errorf("failed to run cmd %q: %w", strings.Join(cmd, " "), err)
		}
	}

	return true, nil
}

func DisableNetworkMode(ctx context.Context, conn remote.RemoteConn) error {
	cmds := [][]string{
		{"sudo", "systemctl", "disable", "ssh"},
		{"sudo", "systemctl", "stop", "ssh"},
		{"sudo", "systemctl", "disable", "avahi-daemon"},
		{"sudo", "systemctl", "stop", "avahi-daemon"},
	}

	for _, cmd := range cmds {
		if out, err := conn.GetCmd(cmd[0], cmd[1:]...).Output(ctx); err != nil {
			return fmt.Errorf("failed to run cmd %q: %w: %s", strings.Join(cmd, " "), err, string(out))
		}
	}

	return nil
}

func getSerial(conn remote.RemoteConn) (string, error) {
	f, err := conn.ReadFile(SerialPath)
	if err != nil {
		return "", fmt.Errorf("failed to get serial number: %w", err)
	}

	serial, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read serial number: %w", err)
	}

	return strings.TrimSpace(string(serial)), nil
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

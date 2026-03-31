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

package adb

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/x/ports"
)

const username = "arduino"

type ADBConnection struct {
	adbPath string
	host    string
}

// Ensures ADBConnection implements the RemoteConn interface at compile time.
var _ remote.RemoteConn = (*ADBConnection)(nil)

var (
	// ErrNotFound is returned when the ADB device is not found.
	ErrNotFound = fmt.Errorf("ADB device not found")
	// ErrDeviceOffline is returned when the ADB device is not reachable.
	// This usually requires a restart of the adbd server daemon on the device.
	ErrDeviceOffline = fmt.Errorf("ADB device is offline")
)

// FromSerial creates an ADBConnection from a device serial number.
// returns an error NotFoundErr if the device is not found, and DeviceOfflineErr if the device is offline.
func FromSerial(serial string, adbPath string) (*ADBConnection, error) {
	if adbPath == "" {
		adbPath = FindAdbPath()
	}

	isConnected := func(serial, adbPath string) (bool, error) {
		cmd, err := paths.NewProcess(nil, adbPath, "-s", serial, "get-state")
		if err != nil {
			return false, fmt.Errorf("failed to create ADB command: %w", err)
		}

		output, err := cmd.RunAndCaptureCombinedOutput(context.TODO())
		if err != nil {
			slog.Error("unable to connect to ADB device", "error", err, "output", string(output), "serial", serial)
			if bytes.Contains(output, []byte("device offline")) {
				return false, ErrDeviceOffline
			} else if bytes.Contains(output, []byte("not found")) {
				return false, ErrNotFound
			}
			return false, fmt.Errorf("failed to get ADB device state: %w: %s", err, output)
		}

		return string(bytes.TrimSpace(output)) == "device", nil
	}

	if connected, err := isConnected(serial, adbPath); err != nil {
		return nil, err
	} else if !connected {
		return nil, fmt.Errorf("device %s is not connected", serial)
	}

	return &ADBConnection{
		adbPath: adbPath,
		host:    serial,
	}, nil
}

func FromHost(host string, adbPath string) (*ADBConnection, error) {
	if adbPath == "" {
		adbPath = FindAdbPath()
	}
	cmd, err := paths.NewProcess(nil, adbPath, "connect", host)
	if err != nil {
		return nil, err
	}
	if out, err := cmd.RunAndCaptureCombinedOutput(context.TODO()); err != nil {
		return nil, fmt.Errorf("failed to connect to ADB host %s: %w: %s", host, err, out)
	}
	return FromSerial(host, adbPath)
}

func (a *ADBConnection) Forward(ctx context.Context, localPort int, remotePort int) error {
	if !ports.IsAvailable(localPort) {
		return remote.ErrPortAvailable
	}

	local := fmt.Sprintf("tcp:%d", localPort)
	remote := fmt.Sprintf("tcp:%d", remotePort)
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "forward", local, remote)
	if err != nil {
		return err
	}
	if out, err := cmd.RunAndCaptureCombinedOutput(ctx); err != nil {
		return fmt.Errorf(
			"failed to forward ADB port %s to %s: %w: %s",
			local,
			remote,
			err,
			out,
		)
	}

	return nil
}

func (a *ADBConnection) ForwardKillAll(ctx context.Context) error {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "forward", "--remove-all")
	if err != nil {
		return err
	}
	if out, err := cmd.RunAndCaptureCombinedOutput(ctx); err != nil {
		return fmt.Errorf("failed to kill all ADB forwarded ports: %w: %s", err, out)
	}
	return nil
}

func (a *ADBConnection) List(path string) ([]remote.FileInfo, error) {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "ls", "-la", path)
	if err != nil {
		return nil, err
	}
	cmd.RedirectStderrTo(os.Stdout)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer output.Close()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() { _ = cmd.Wait() }()

	r := bufio.NewReader(output)
	_, err = r.ReadBytes('\n') // Skip the first line
	if err != nil {
		return nil, err
	}

	var files []remote.FileInfo
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		parts := bytes.Split(line, []byte(" "))
		name := string(parts[len(parts)-1])
		if name == "." || name == ".." {
			continue
		}
		files = append(files, remote.FileInfo{
			Name:  name,
			IsDir: line[0] == 'd',
		})
	}

	return files, nil
}

func (a *ADBConnection) Stats(p string) (remote.FileInfo, error) {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "file", p)
	if err != nil {
		return remote.FileInfo{}, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return remote.FileInfo{}, err
	}
	defer output.Close()
	if err := cmd.Start(); err != nil {
		return remote.FileInfo{}, err
	}
	defer func() { _ = cmd.Wait() }()

	r := bufio.NewReader(output)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return remote.FileInfo{}, err
	}

	line = bytes.TrimSpace(line)
	parts := bytes.Split(line, []byte(":"))
	if len(parts) < 2 {
		return remote.FileInfo{}, fmt.Errorf("unexpected file command output: %s", line)
	}

	name := string(bytes.TrimSpace(parts[0]))
	other := string(bytes.TrimSpace(parts[1]))

	if strings.Contains(other, "cannot open") {
		return remote.FileInfo{}, fs.ErrNotExist
	}

	return remote.FileInfo{
		Name:  path.Base(name),
		IsDir: other == "directory",
	}, nil
}

func (a *ADBConnection) ReadFile(path string) (io.ReadCloser, error) {
	return adbReadFile(a, path)
}

func (a *ADBConnection) WriteFile(r io.Reader, path string) error {
	return adbWriteFile(a, r, path)
}

func (a *ADBConnection) MkDirAll(path string) error {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "install", "-o", username, "-g", username, "-m", "755", "-d", path)
	if err != nil {
		return err
	}
	stdout, err := cmd.RunAndCaptureCombinedOutput(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create directory %q: %w: %s", path, err, string(stdout))
	}
	return nil
}

func (a *ADBConnection) Remove(path string) error {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "rm", "-r", path) // nolint:gosec
	if err != nil {
		return err
	}
	stdout, err := cmd.RunAndCaptureCombinedOutput(context.Background())
	if err != nil {
		return fmt.Errorf("failed to remove path %q: %w: %s", path, err, string(stdout))
	}
	return nil
}

type ADBCommand struct {
	cmd *paths.Process
	err error
}

func (a *ADBConnection) GetCmd(cmd string, args ...string) remote.Cmder {
	for i, arg := range args {
		if strings.Contains(arg, " ") {
			args[i] = fmt.Sprintf("%q", arg)
		}
	}

	// TODO: fix command injection vulnerability
	var cmds []string
	cmds = append(cmds, a.adbPath, "-s", a.host, "shell", cmd)
	if len(args) > 0 {
		cmds = append(cmds, args...)
	}

	command, err := paths.NewProcess(nil, cmds...)
	return &ADBCommand{cmd: command, err: err}
}

func (a *ADBCommand) Run(ctx context.Context) error {
	if a.err != nil {
		return fmt.Errorf("failed to create command: %w", a.err)
	}

	return a.cmd.RunWithinContext(ctx)
}

func (a *ADBCommand) Output(ctx context.Context) ([]byte, error) {
	if a.err != nil {
		return nil, fmt.Errorf("failed to create command: %w", a.err)
	}

	return a.cmd.RunAndCaptureCombinedOutput(ctx)
}

func (a *ADBCommand) Interactive() (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
	if a.err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create command: %w", a.err)
	}

	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := a.cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := a.cmd.Start(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	return stdin, stdout, stderr, func() error {
		if err := stdout.Close(); err != nil {
			return fmt.Errorf("failed to close stdout pipe: %w", err)
		}
		if err := stderr.Close(); err != nil {
			return fmt.Errorf("failed to close stderr pipe: %w", err)
		}
		if err := a.cmd.Wait(); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
		return nil
	}, nil
}

func FindAdbPath() string {
	var adbPath = "adb"

	// Attempt to find the adb path in the Arduino15 directory
	const arduino15adbPath = "packages/arduino/tools/adb/32.0.0/adb"
	var path string
	switch runtime.GOOS {
	case "darwin":
		user, err := user.Current()
		if err != nil {
			slog.Warn("Unable to get current user", "error", err)
			break
		}
		path = filepath.Join(user.HomeDir, "/Library/Arduino15/", arduino15adbPath)
	case "linux":
		user, err := user.Current()
		if err != nil {
			slog.Warn("Unable to get current user", "error", err)
			break
		}
		path = filepath.Join(user.HomeDir, ".arduino15/", arduino15adbPath)
	case "windows":
		user, err := user.Current()
		if err != nil {
			slog.Warn("Unable to get current user", "error", err)
			break
		}
		path = filepath.Join(user.HomeDir, "AppData/Local/Arduino15/", arduino15adbPath)
		path += ".exe"
	}
	s, err := os.Stat(path)
	if err == nil && !s.IsDir() {
		adbPath = path
	}

	slog.Debug("get adb path", "path", adbPath)

	return adbPath
}

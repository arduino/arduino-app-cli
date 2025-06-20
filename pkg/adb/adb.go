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
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/sirupsen/logrus"
)

type ADBConnection struct {
	host    string
	adbPath string
	User    string
}

func FromFQBN(ctx context.Context, fqbn string, adbPath string) (*ADBConnection, error) {
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
		Timeout:  1000, // 1 seconds
		Fqbn:     fqbn,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get info for FQBN %s: %w", fqbn, err)
	}
	if ports := list.GetPorts(); len(ports) != 0 {
		if port := ports[0].GetPort(); port != nil {
			if serial := port.GetHardwareId(); serial != "" {
				return FromSerial(serial, adbPath), nil
			}
		}
	}
	return nil, fmt.Errorf("no hardware ID found for FQBN %s", fqbn)
}

func FromSerial(serial string, adbPath string) *ADBConnection {
	if adbPath == "" {
		adbPath = findAdbPath()
	}
	return &ADBConnection{
		host:    serial,
		adbPath: adbPath,
		User:    "arduino",
	}
}

func FromHost(host string, adbPath string) (*ADBConnection, error) {
	if adbPath == "" {
		adbPath = findAdbPath()
	}
	if err := exec.Command(adbPath, "connect", host).Run(); err != nil {
		return nil, fmt.Errorf("failed to connect to ADB host %s: %w", host, err)
	}
	return &ADBConnection{
		host:    host,
		adbPath: adbPath,
		User:    "arduino",
	}, nil
}

type FileInfo struct {
	Name  string
	IsDir bool
}

func (a *ADBConnection) List(path string) ([]FileInfo, error) {
	cmd := a.GetCmd(context.TODO(), "ls", "-la", path) // nolint:gosec
	cmd.Stderr = os.Stdout
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer output.Close()
	slog.Debug("adb List", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	r := bufio.NewReader(output)
	_, err = r.ReadBytes('\n') // Skip the first line
	if err != nil {
		return nil, err
	}

	var files []FileInfo
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
		files = append(files, FileInfo{
			Name:  name,
			IsDir: line[0] == 'd',
		})
	}

	return files, nil
}

func (a *ADBConnection) Stats(path string) (FileInfo, error) {
	cmd := a.GetCmd(context.TODO(), "file", path) // nolint:gosec
	output, err := cmd.StdoutPipe()
	if err != nil {
		return FileInfo{}, err
	}
	defer output.Close()
	if err := cmd.Start(); err != nil {
		return FileInfo{}, err
	}

	r := bufio.NewReader(output)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return FileInfo{}, err
	}

	line = bytes.TrimSpace(line)
	parts := bytes.Split(line, []byte(":"))
	if len(parts) < 2 {
		return FileInfo{}, fmt.Errorf("unexpected file command output: %s", line)
	}

	name := string(bytes.TrimSpace(parts[0]))
	other := string(bytes.TrimSpace(parts[1]))

	if strings.Contains(other, "cannot open") {
		return FileInfo{}, fs.ErrNotExist
	}

	return FileInfo{
		Name:  name,
		IsDir: other == "directory",
	}, nil
}

func (a *ADBConnection) CatOut(path string) (io.ReadCloser, error) {
	cmd := a.GetCmd(context.TODO(), "cat", path) // nolint:gosec
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	slog.Debug("CatOut", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return output, nil
}

func (a *ADBConnection) CatIn(r io.Reader, path string) error {
	cmd := a.GetCmd(context.TODO(), "cat", ">", path) // nolint:gosec
	cmd.Stdin = r
	out, err := cmd.CombinedOutput()
	slog.Debug("adb CatIn", "cmd", cmd.String(), "out", string(out))
	if err != nil {
		return err
	}
	return nil
}

func (a *ADBConnection) MkDirAll(path string) error {
	cmd := a.GetCmd(context.TODO(), "mkdir", "-p", path) // nolint:gosec
	out, err := cmd.CombinedOutput()
	slog.Debug("adb MkDirAll", "cmd", cmd.String(), "out", string(out))
	if err != nil {
		return err
	}
	return nil
}

func (a *ADBConnection) Remove(path string) error {
	cmd := a.GetCmd(context.TODO(), "rm", "-r", path) // nolint:gosec
	out, err := cmd.CombinedOutput()
	slog.Debug("adb Remove", "cmd", cmd.String(), "out", string(out))
	if err != nil {
		return err
	}
	return nil
}

func (a *ADBConnection) GetCmd(ctx context.Context, args ...string) *exec.Cmd {
	for i, arg := range args {
		if strings.Contains(arg, " ") {
			args[i] = fmt.Sprintf("%q", arg)
		}
	}

	// TODO: fix command injection vulnerability
	return exec.CommandContext(ctx, a.adbPath, "-s", a.host, "shell", "su", "-", a.User, "-c", fmt.Sprintf("%q", fmt.Sprintf("sh -c %q", strings.Join(args, " ")))) // nolint:gosec
}

func (a *ADBConnection) Run(args ...string) (string, error) {
	cmd := a.GetCmd(context.Background(), args...)
	output, err := cmd.CombinedOutput() // nolint:gosec
	if err != nil {
		return "", fmt.Errorf("failed to run command %q: %w", cmd.String(), err)
	}
	return string(output), nil
}

func findAdbPath() string {
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

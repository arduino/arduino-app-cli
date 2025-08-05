package adb

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

const username = "arduino"

type ADBConnection struct {
	adbPath string
	host    string
}

// Ensures ADBConnection implements the RemoteConn interface at compile time.
var _ remote.RemoteConn = (*ADBConnection)(nil)

func FromSerial(serial string, adbPath string) (*ADBConnection, error) {
	if adbPath == "" {
		adbPath = FindAdbPath()
	}

	return &ADBConnection{
		host:    serial,
		adbPath: adbPath,
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
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to connect to ADB host %s: %w", host, err)
	}
	return FromSerial(host, adbPath)
}

func (a *ADBConnection) Forward(ctx context.Context, remotePort int) (int, error) {
	hostAvailablePort, err := getAvailablePort()
	if err != nil {
		return 0, fmt.Errorf("failed to find an available port: %w", err)
	}

	local := fmt.Sprintf("tcp:%d", hostAvailablePort)
	remote := fmt.Sprintf("tcp:%d", remotePort)
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "forward", local, remote)
	if err != nil {
		return hostAvailablePort, err
	}
	if err := cmd.RunWithinContext(ctx); err != nil {
		return hostAvailablePort, fmt.Errorf(
			"failed to forward ADB port %s to %s: %w",
			local,
			remote,
			err,
		)
	}
	return hostAvailablePort, nil
}

func (a *ADBConnection) ForwardKillAll(ctx context.Context) error {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "killforward-all")
	if err != nil {
		return err
	}
	if err := cmd.RunWithinContext(ctx); err != nil {
		return fmt.Errorf("failed to kill all ADB forwarded ports: %w", err)
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

func (a *ADBConnection) Stats(path string) (remote.FileInfo, error) {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "file", path)
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
		Name:  name,
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

	command, _ := paths.NewProcess(nil, cmds...)
	return &ADBCommand{cmd: command}
}

func (a *ADBCommand) Run(ctx context.Context) error {
	return a.cmd.RunWithinContext(ctx)
}

func (a *ADBCommand) Output(ctx context.Context) ([]byte, error) {
	return a.cmd.RunAndCaptureCombinedOutput(ctx)
}

func (a *ADBCommand) Interactive() (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
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

func isPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

func getRandomPort() int {
	port := 1000 + rand.IntN(9000) // nolint:gosec
	return port
}

const forwardPortAttempts = 10

func getAvailablePort() (int, error) {
	tried := make(map[int]any, forwardPortAttempts)
	for len(tried) < forwardPortAttempts {
		port := getRandomPort()
		if _, seen := tried[port]; seen {
			continue
		}
		tried[port] = struct{}{}

		if isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range 1000-9999 after %d attempts", forwardPortAttempts)
}

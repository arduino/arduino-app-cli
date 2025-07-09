package adb

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
)

type ADBConnection struct {
	adbPath string
	host    string
	client  *ssh.SSHConnection
}

// Ensures ADBConnection implements the RemoteConn interface at compile time.
var _ remote.RemoteConn = (*ADBConnection)(nil)

const forwardPortAttempts = 10

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

func FromSerial(serial string, adbPath string) (*ADBConnection, error) {
	if adbPath == "" {
		adbPath = findAdbPath()
	}
	// TODO: we should try multiple ports here.
	if err := exec.Command(adbPath, "-s", serial, "forward", "tcp:2222", "tcp:22").Run(); err != nil {
		panic(fmt.Errorf("failed to forward ADB port for serial %s: %w", serial, err))
	}

	sshConn, err := ssh.FromHost("arduino", "arduino", "127.0.0.1:2222")
	if err != nil {
		panic(fmt.Errorf("failed to create SSH connection for serial %s: %w", serial, err))
	}
	return &ADBConnection{
		client:  sshConn,
		host:    serial,
		adbPath: adbPath,
	}, nil
}

func FromHost(host string, adbPath string) (*ADBConnection, error) {
	if adbPath == "" {
		adbPath = findAdbPath()
	}
	if err := exec.Command(adbPath, "connect", host).Run(); err != nil {
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
	if err := exec.CommandContext(ctx, a.adbPath, "-s", a.host, "forward", local, remote).Run(); err != nil { // nolint:gosec
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
	if err := exec.CommandContext(ctx, a.adbPath, "-s", a.host, "killforward-all").Run(); err != nil { // nolint:gosec
		return fmt.Errorf("failed to kill all ADB forwarded ports: %w", err)
	}
	return nil
}

func (a *ADBConnection) List(path string) ([]remote.FileInfo, error) {
	return a.client.List(path)
}

func (a *ADBConnection) Stats(path string) (remote.FileInfo, error) {
	return a.client.Stats(path)
}

func (a *ADBConnection) ReadFile(path string) (io.ReadCloser, error) {
	return a.client.ReadFile(path)
}

func (a *ADBConnection) WriteFile(r io.Reader, path string) error {
	return a.client.WriteFile(r, path)
}

func (a *ADBConnection) MkDirAll(path string) error {
	return a.client.MkDirAll(path)
}

func (a *ADBConnection) Remove(path string) error {
	return a.client.Remove(path)
}

type ADBCommand struct {
	cmd *exec.Cmd
}

func (a *ADBConnection) GetCmd(ctx context.Context, cmd string, args ...string) remote.Cmder {
	for i, arg := range args {
		if strings.Contains(arg, " ") {
			args[i] = fmt.Sprintf("%q", arg)
		}
	}

	// TODO: fix command injection vulnerability
	var cmds []string
	cmds = append(cmds, "-s", a.host, "shell", cmd)
	cmds = append(cmds, args...)

	cmdd := exec.CommandContext(ctx, a.adbPath, cmds...) // nolint:gosec
	return &ADBCommand{
		cmd: cmdd,
	}
}

func (a *ADBCommand) Run() error {
	return a.cmd.Run()
}

func (a *ADBCommand) Output() ([]byte, error) {
	return a.cmd.CombinedOutput()
}

func (a *ADBCommand) Interactive() (io.WriteCloser, io.Reader, remote.Closer, error) {
	a.cmd.Stderr = a.cmd.Stdout // Redirect stderr to stdout
	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := a.cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	return stdin, stdout, func() error {
		if err := stdout.Close(); err != nil {
			return fmt.Errorf("failed to close stdout pipe: %w", err)
		}
		if err := a.cmd.Wait(); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
		return nil
	}, nil
}

func (a *ADBConnection) GetCmdAsUser(ctx context.Context, user string, cmd string, args ...string) remote.Cmder {
	return a.client.GetCmdAsUser(ctx, user, cmd, args...)
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

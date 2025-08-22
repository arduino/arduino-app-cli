package local

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

type LocalConnection struct{}

// Ensures LocalConnection implements the RemoteConn interface at compile time.
var _ remote.RemoteConn = (*LocalConnection)(nil)

func (a *LocalConnection) Forward(ctx context.Context, remotePort int) (int, error) {
	// Locally we don't need to forward ports.
	return remotePort, nil
}

func (a *LocalConnection) ForwardKillAll(ctx context.Context) error {
	return nil
}

func (a *LocalConnection) List(path string) ([]remote.FileInfo, error) {
	dirs, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", path, err)
	}

	return f.Map(dirs, func(d fs.DirEntry) remote.FileInfo {
		return remote.FileInfo{
			Name:  d.Name(),
			IsDir: d.IsDir(),
		}
	}), nil
}

func (a *LocalConnection) Stats(path string) (remote.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return remote.FileInfo{}, fmt.Errorf("failed to get stats for path %q: %w", path, err)
	}

	return remote.FileInfo{
		Name:  info.Name(),
		IsDir: info.IsDir(),
	}, nil
}

func (a *LocalConnection) ReadFile(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (a *LocalConnection) WriteFile(r io.Reader, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("failed to write to file %q: %w", path, err)
	}

	return nil
}

func (a *LocalConnection) MkDirAll(path string) error {
	return os.MkdirAll(path, 0700)
}

func (a *LocalConnection) Remove(path string) error {
	return os.RemoveAll(path)
}

type ADBCommand struct {
	cmd *paths.Process
	err error
}

func (a *LocalConnection) GetCmd(cmd string, args ...string) remote.Cmder {
	cmds := make([]string, 0, 1+len(args))
	cmds = append(cmds, cmd)
	cmds = append(cmds, args...)

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

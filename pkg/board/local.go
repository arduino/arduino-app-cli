package board

import (
	"context"
	"io"
	"os/exec"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

type LocalCmder struct{}

func (l *LocalCmder) GetCmd(ctx context.Context, cmd string, args ...string) remote.Cmder {
	return &LocalCmd{
		cmd: exec.CommandContext(ctx, cmd, args...),
	}
}

func (l *LocalCmder) GetCmdAsUser(ctx context.Context, user string, cmd string, args ...string) remote.Cmder {
	return l.GetCmd(ctx, cmd, args...)
}

type LocalCmd struct {
	cmd *exec.Cmd
}

func (l *LocalCmd) Run() error {
	return l.cmd.Run()
}

func (l *LocalCmd) Output() ([]byte, error) {
	return l.cmd.Output()
}

func (l *LocalCmd) Interactive() (io.WriteCloser, io.Reader, remote.Closer, error) {
	l.cmd.Stderr = l.cmd.Stdout // Redirect stderr to stdout
	stdin, err := l.cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := l.cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	if err := l.cmd.Start(); err != nil {
		return nil, nil, nil, err
	}

	return stdin, stdout, func() error {
		_ = stdout.Close()
		if err := l.cmd.Wait(); err != nil {
			return err
		}
		return nil
	}, nil
}

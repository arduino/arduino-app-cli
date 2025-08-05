package board

import (
	"context"
	"io"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

type LocalCmder struct{}

func (l *LocalCmder) GetCmd(cmd string, args ...string) remote.Cmder {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, cmd)
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}
	command, _ := paths.NewProcess(nil, cmdArgs...)
	return &LocalCmd{cmd: command}
}

func (l *LocalCmder) GetCmdAsUser(user string, cmd string, args ...string) remote.Cmder {
	return l.GetCmd(cmd, args...)
}

type LocalCmd struct {
	cmd *paths.Process
}

func (l *LocalCmd) Run(ctx context.Context) error {
	return l.cmd.RunWithinContext(ctx)
}

func (l *LocalCmd) Output(ctx context.Context) ([]byte, error) {
	return l.cmd.RunAndCaptureCombinedOutput(ctx)
}

func (l *LocalCmd) Interactive() (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
	stdin, err := l.cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stdout, err := l.cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stderr, err := l.cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if err := l.cmd.Start(); err != nil {
		return nil, nil, nil, nil, err
	}

	return stdin, stdout, stderr, func() error {
		_ = stdout.Close()
		_ = stderr.Close()
		if err := l.cmd.Wait(); err != nil {
			return err
		}
		return nil
	}, nil
}

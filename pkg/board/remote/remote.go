// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

var ErrPortAvailable = fmt.Errorf("port is not available")

type FileInfo struct {
	Name      string
	IsDir     bool
	IsSymlink bool
}

type RemoteConn interface {
	FS
	RemoteShell // TODO: should be removed after refactoring.
	Forwarder
	RemoteTransfer
}

type FS interface {
	List(path string) ([]FileInfo, error)
	MkDirAll(path string) error
	WriteFile(data io.Reader, path string) error
	ReadFile(path string) (io.ReadCloser, error)
	Remove(path string) error
	Stats(path string) (FileInfo, error)
}

type RemoteShell interface {
	GetCmd(cmd string, args ...string) Cmder
}

type Forwarder interface {
	Forward(ctx context.Context, localPort int, remotePort int) error
	ForwardKillAll(ctx context.Context) error
}

type Closer func() error

type Cmder interface {
	Run(ctx context.Context) error
	Output(ctx context.Context) ([]byte, error)
	Interactive() (io.WriteCloser, io.Reader, io.Reader, Closer, error)
}

type RemoteTransfer interface {
	// Push copies a file or directory from the local path to the remote path.
	// The remote path should always specify the final destination path, and not
	// the parent directory, even if it exist.
	// The remote path could instead be different from the local path, and that will
	// rename while copying.
	Push(ctx context.Context, local, remote string) error
}

// ErrConnLost is returned when the connection to the board drops. Data read
// before the drop can be incomplete.
var ErrConnLost = errors.New("connection to the board lost")

// CmdError classifies the exit error of a remote command from its stderr, so
// that a caller can tell a missing file from an unreachable board.
func CmdError(err error, stderr []byte) error {
	if err == nil {
		return nil
	}

	msg := string(bytes.TrimSpace(stderr))
	switch {
	case strings.Contains(msg, "device offline"), strings.Contains(msg, "error: device"),
		strings.Contains(msg, "closed by remote host"), strings.Contains(msg, "connection reset"):
		return fmt.Errorf("%w: %s", ErrConnLost, msg)
	// "file" prints "cannot open" with the reason in brackets, so the permission
	// case must come first.
	case strings.Contains(msg, "Permission denied"):
		return fmt.Errorf("%w: %s", fs.ErrPermission, msg)
	case strings.Contains(msg, "No such file or directory"), strings.Contains(msg, "cannot open"):
		return fmt.Errorf("%w: %s", fs.ErrNotExist, msg)
	case msg == "":
		return err
	default:
		return fmt.Errorf("%w: %s", err, msg)
	}
}

// OpenOutput returns a reader over the output of a started command. It waits
// for the first byte, so that a command that fails at once reports it here.
func OpenOutput(r io.Reader, exitErr func() error) (io.Reader, error) {
	buffered := bufio.NewReader(r)
	if _, err := buffered.Peek(1); err != nil {
		// No output at all: the command failed, or the file is empty.
		if err := exitErr(); err != nil {
			return nil, err
		}
	}
	return buffered, nil
}

// WithCloser is a helper to create an io.ReadCloser from an io.Reader
// and a close function.
type WithCloser struct {
	io.Reader
	CloseFun func() error
}

func (w WithCloser) Close() error {
	if w.CloseFun != nil {
		return w.CloseFun()
	}
	return nil
}

// ShellQuote quotes s so it can be safely used as a single argument in a POSIX
// shell command. It wraps the value in single quotes, which prevents the shell
// from interpreting special characters such as '$', backticks or backslashes.
// Any embedded single quote is escaped using the standard '\” idiom.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

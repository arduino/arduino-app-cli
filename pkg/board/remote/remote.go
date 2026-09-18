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

// ErrConnLost is returned when the connection to the board drops during an
// operation. Data read before the drop can be incomplete.
var ErrConnLost = errors.New("connection to the board lost")

// ReadError classifies the exit error of a remote command from its stderr, so
// that a caller can tell a missing file from an unreachable board.
func ReadError(err error, stderr []byte) error {
	if err == nil {
		return nil
	}

	msg := string(bytes.TrimSpace(stderr))
	switch {
	case strings.Contains(msg, "device offline"), strings.Contains(msg, "error: device"),
		strings.Contains(msg, "closed by remote host"), strings.Contains(msg, "connection reset"):
		return fmt.Errorf("%w: %s", ErrConnLost, msg)
	// "cannot open" comes from "file", which reports the reason in brackets, so
	// the permission case must be tested first.
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

// ReadResult ends a remote read command. done is true when the output was read
// to its end, and the command outcome is the result of the read. A false done
// means the reader was closed early: the command must be stopped without
// waiting for its output, and its outcome is not a read failure.
type ReadResult func(done bool) error

// StartRead returns a reader over an already started remote read command. It
// waits for the first chunk of output, so a command that fails at once reports
// the error here, like os.Open does. A failure later in the stream replaces the
// final io.EOF, so that a truncated read cannot pass as a complete one.
func StartRead(r io.Reader, result ReadResult) (io.ReadCloser, error) {
	buffered := bufio.NewReader(r)
	if _, err := buffered.Peek(1); err != nil {
		// No output at all: either the file is empty, or the command failed.
		if cerr := result(true); cerr != nil {
			return nil, cerr
		}
		if !errors.Is(err, io.EOF) {
			return nil, err
		}
		return WithCloser{Reader: bytes.NewReader(nil)}, nil
	}

	return &cmdReader{r: buffered, result: result}, nil
}

// cmdReader reads the output of a remote command. The command result is known
// only at the end of the stream, so the reader waits for it there, and not only
// at close time, which most callers ignore.
type cmdReader struct {
	r      io.Reader
	result ReadResult

	done   bool
	closed bool
	err    error
}

func (c *cmdReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if errors.Is(err, io.EOF) {
		c.done = true
		if result := c.Close(); result != nil {
			return n, result
		}
	}
	return n, err
}

func (c *cmdReader) Close() error {
	if !c.closed {
		c.closed = true
		c.err = c.result(c.done)
	}
	return c.err
}

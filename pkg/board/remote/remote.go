// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote

import (
	"context"
	"fmt"
	"io"
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

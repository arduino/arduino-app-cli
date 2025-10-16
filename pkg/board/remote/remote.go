// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package remote

import (
	"context"
	"fmt"
	"io"
)

var ErrPortAvailable = fmt.Errorf("port is not available")

type FileInfo struct {
	Name  string
	IsDir bool
}

type RemoteConn interface {
	FS
	RemoteShell // TODO: should be removed after refactoring.
	Forwarder
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

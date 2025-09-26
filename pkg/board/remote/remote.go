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

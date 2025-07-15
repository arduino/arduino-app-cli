package remote

import (
	"context"
	"io"
)

type FileInfo struct {
	Name  string
	IsDir bool
}

type RemoteConn interface {
	RemoteFs
	RemoteShell
	Forwarder
}

type RemoteFs interface {
	List(path string) ([]FileInfo, error)
	MkDirAll(path string) error
	WriteFile(data io.Reader, path string) error
	ReadFile(path string) (io.ReadCloser, error)
	Remove(path string) error
	Stats(path string) (FileInfo, error)
}

type RemoteShell interface {
	GetCmd(ctx context.Context, cmd string, args ...string) Cmder
}

type Forwarder interface {
	Forward(ctx context.Context, remotePort int) (int, error)
	ForwardKillAll(ctx context.Context) error
}

type Closer func() error

type Cmder interface {
	Run() error
	Output() ([]byte, error)
	Interactive() (io.WriteCloser, io.Reader, Closer, error)
}

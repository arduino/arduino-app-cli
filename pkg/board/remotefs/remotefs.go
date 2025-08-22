package remotefs

import (
	"io"
	"io/fs"
	"path"
	"time"

	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

type RemoteFS struct {
	base string

	conn remote.FS
}

func New(base string, conn remote.FS) RemoteFS {
	return RemoteFS{
		base: base,
		conn: conn,
	}
}

func (a RemoteFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}

	fullPath := path.Join(a.base, name)
	stats, err := a.conn.Stats(fullPath)
	if err != nil {
		return nil, err
	}
	if stats.IsDir {
		return &RemoteDir{name: name, base: a.base, conn: a.conn}, nil
	}

	return &RemoteFile{name: name, base: a.base, conn: a.conn}, nil
}

type RemoteFSWriter struct {
	RemoteFS
}

func (a RemoteFS) ToWriter() RemoteFSWriter {
	return RemoteFSWriter{
		RemoteFS: a,
	}
}

func (a RemoteFSWriter) MkDirAll(p string) error {
	return a.conn.MkDirAll(path.Join(a.base, p))
}

func (a RemoteFSWriter) WriteFile(p string, data io.ReadCloser) error {
	return a.conn.WriteFile(data, path.Join(a.base, p))
}

func (a RemoteFSWriter) RmFile(p string) error {
	return a.conn.Remove(path.Join(a.base, p))
}

type RemoteFile struct {
	name string
	base string

	read io.ReadCloser

	conn remote.FS
}

func (a *RemoteFile) Read(p []byte) (n int, err error) {
	if a.read == nil {
		r, err := a.conn.ReadFile(path.Join(a.base, a.name))
		if err != nil {
			return 0, err
		}
		a.read = r
	}
	return a.read.Read(p)
}

func (a RemoteFile) Close() error {
	if a.read == nil {
		return nil
	}

	if err := a.read.Close(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (a RemoteFile) Stat() (fs.FileInfo, error) {
	return &RemoteFileInfo{name: path.Base(a.name)}, nil
}

type RemoteFileInfo struct {
	name string

	isDir bool
}

func (a RemoteFileInfo) Name() string {
	return a.name
}

func (a RemoteFileInfo) Size() int64 {
	// TODO: implement size
	return 0
}

func (a RemoteFileInfo) Mode() fs.FileMode {
	if a.isDir {
		return fs.ModeDir
	}
	return 0
}

func (a RemoteFileInfo) ModTime() time.Time {
	// TODO: implement mod time
	return time.Time{}
}

func (a RemoteFileInfo) IsDir() bool {
	return a.isDir
}

func (a RemoteFileInfo) Sys() any {
	return nil
}

type RemoteDir struct {
	name string
	base string

	files []remote.FileInfo
	valid bool
	conn  remote.FS
}

func (a *RemoteDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if !a.valid {
		files, err := a.conn.List(path.Join(a.base, a.name))
		if err != nil {
			return nil, err
		}
		a.files = files
		a.valid = true
	}

	if n > 0 && len(a.files) == 0 {
		return nil, io.EOF
	}

	if n <= 0 || n > len(a.files) {
		n = len(a.files)
	}
	files, rest := a.files[:n], a.files[n:]
	a.files = rest

	return f.Map(files, func(file remote.FileInfo) fs.DirEntry {
		return RemoteDirEntry{
			name:  file.Name,
			isDir: file.IsDir,
		}
	}), nil
}

func (a RemoteDir) Stat() (fs.FileInfo, error) {
	return &RemoteFileInfo{name: path.Base(a.name), isDir: true}, nil
}

func (a RemoteDir) Close() error {
	// No resources to close
	return nil
}

func (a RemoteDir) Read(p []byte) (n int, err error) {
	// No data to read
	panic("cannot read a folder")
}

type RemoteDirEntry struct {
	name string

	isDir bool
}

func (a RemoteDirEntry) Name() string {
	return a.name
}
func (a RemoteDirEntry) IsDir() bool {
	return a.isDir
}
func (a RemoteDirEntry) Type() fs.FileMode {
	if a.isDir {
		return fs.ModeDir
	}
	return 0
}

func (a RemoteDirEntry) Info() (fs.FileInfo, error) {
	return &RemoteFileInfo{name: a.name, isDir: a.isDir}, nil
}

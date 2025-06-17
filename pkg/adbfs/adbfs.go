package adbfs

import (
	"io"
	"io/fs"
	"path"
	"time"

	"github.com/arduino/arduino-app-cli/pkg/adb"
)

type AdbFS struct {
	base string

	adb *adb.ADBConnection
}

func NewAdbFS(base string, adb *adb.ADBConnection) AdbFS {
	return AdbFS{
		base: base,
		adb:  adb,
	}
}

func (a AdbFS) Open(name string) (fs.File, error) {
	name = path.Join(a.base, name)
	stats, err := a.adb.Stats(name)
	if err != nil {
		return nil, err
	}
	if stats.IsDir {
		return AdbReadDirFile{name: name, adb: a.adb}, nil
	}

	return &AdbFile{name: name, adb: a.adb}, nil
}

type AdbFSWriter struct {
	AdbFS
}

func (a AdbFS) ToWriter() AdbFSWriter {
	return AdbFSWriter{
		AdbFS: a,
	}
}

func (a AdbFSWriter) MkDirAll(p string) error {
	return a.adb.MkDirAll(path.Join(a.base, p))
}

func (a AdbFSWriter) WriteFile(p string, data io.ReadCloser) error {
	return a.adb.CatIn(data, path.Join(a.base, p))
}

func (a AdbFSWriter) RmFile(p string) error {
	return a.adb.Remove(path.Join(a.base, p))
}

type AdbFile struct {
	name string
	read io.ReadCloser

	adb *adb.ADBConnection
}

func (a *AdbFile) Read(p []byte) (n int, err error) {
	if a.read == nil {
		r, err := a.adb.CatOut(a.name)
		if err != nil {
			return 0, err
		}
		a.read = r
	}
	return a.read.Read(p)
}

func (a AdbFile) Close() error {
	if a.read == nil {
		return nil
	}
	return a.read.Close()
}

func (a AdbFile) Stat() (fs.FileInfo, error) {
	return &AdbFileInfo{name: a.name}, nil
}

type AdbFileInfo struct {
	name  string
	isDir bool
}

func (a AdbFileInfo) Name() string {
	return a.name
}

func (a AdbFileInfo) Size() int64 {
	panic("not implemented")
}

func (a AdbFileInfo) Mode() fs.FileMode {
	if a.isDir {
		return fs.ModeDir
	}
	return 0
}

func (a AdbFileInfo) ModTime() time.Time {
	panic("not implemented")
}

func (a AdbFileInfo) IsDir() bool {
	return a.isDir
}

func (a AdbFileInfo) Sys() any {
	panic("not implemented")
}

type AdbReadDirFile struct {
	name string

	adb *adb.ADBConnection
}

func (a AdbReadDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	files, err := a.adb.List(a.name)
	if err != nil {
		return nil, err
	}

	entries := make([]fs.DirEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, AdbDirEntry{
			name:  file.Name,
			isDir: file.IsDir,
		})
	}

	if n > 0 && len(entries) > n {
		return entries[:n], nil
	}
	return entries, nil
}

func (a AdbReadDirFile) Stat() (fs.FileInfo, error) {
	return &AdbFileInfo{name: a.name, isDir: true}, nil
}

func (a AdbReadDirFile) Close() error {
	// No resources to close
	return nil
}

func (a AdbReadDirFile) Read(p []byte) (n int, err error) {
	// No data to read
	panic("cannot read a folder")
}

type AdbDirEntry struct {
	name  string
	isDir bool
}

func (a AdbDirEntry) Name() string {
	return a.name
}
func (a AdbDirEntry) IsDir() bool {
	return a.isDir
}
func (a AdbDirEntry) Type() fs.FileMode {
	if a.isDir {
		return fs.ModeDir
	}
	return 0
}

func (a AdbDirEntry) Info() (fs.FileInfo, error) {
	return &AdbFileInfo{name: a.name}, nil
}

package main

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/arduino/arduino-app-cli/pkg/adbfs"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

type FileInfo struct {
	Name  string
	Path  string
	IsDir bool
}

func (a *App) PullSync(localPath string, boardPath string) error {
	return adbfs.SyncFS(adbfs.OsFSWriter{Base: localPath}, adbfs.AdbFS{Base: boardPath})
}

func (a *App) PushSync(localPath string, boardPath string) error {
	return adbfs.SyncFS(adbfs.AdbFSWriter{Base: boardPath}, os.DirFS(localPath))
}

func (a *App) ListFiles(path string) ([]FileInfo, error) {
	var files []FileInfo
	err := fs.WalkDir(os.DirFS(path), ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			files = append(files, FileInfo{Name: info.Name(), Path: path, IsDir: true})
		} else {
			files = append(files, FileInfo{Name: info.Name(), Path: path, IsDir: false})
		}
		return nil
	})
	return files, err
}

func (a *App) ReadFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *App) WriteFile(path string, data []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) Rename(srcPath string, dstPath string) error {
	return os.Rename(srcPath, dstPath)
}

func (a *App) Remove(path string) error {
	return os.RemoveAll(path)
}

func (a *App) Copy(srcPath string, dstPath string) error {
	return os.CopyFS(dstPath, os.DirFS(srcPath))
}

func (a *App) MkDirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

func (a *App) MkTempDir(name string) (string, error) {
	tmp := filepath.Join(os.TempDir(), "arduino_adbfs", name)
	if err := a.MkDirAll(tmp); err != nil {
		return "", err
	}
	return tmp, nil
}

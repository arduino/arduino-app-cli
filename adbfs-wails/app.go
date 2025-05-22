package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/arduino/arduino-create-agent/updater"
	"github.com/arduino/arduino-app-cli/pkg/adbfs"
)

// App struct
type App struct {
	ctx           context.Context
	updateBaseUrl string
}

// NewApp creates a new App application struct
func NewApp(updateBaseUrl string) *App {
	return &App{
		updateBaseUrl: updateBaseUrl,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	a.ctx = ctx
	fmt.Println("Shutted down wails")
}

func (a *App) restartWithPath(restartPath string) error {
	cmd := exec.Command(restartPath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	// FIXME: if the other app starts before the current one exits, it will fail in the main.go
	// with open /home/dido/code/bmci-labs/orchestrator/adbfs-wails/build/bin/test-wails-temp: text file busy
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("Failed to restart: %w", err)
	}
	runtime.Quit(a.ctx)
	return nil
}

func (a *App) GetVersion() string {
	return version
}

func (a *App) CheckAndApplyUpdate() error {
	// FIXME: is the the "/" after the "http://127.0.0.1:3001/" is missing the update fails. Full URL http://127.0.0.1:3001/arduinoAppsLabs/Stable/linux-amd64.json
	restartPath, err := updater.CheckForUpdates(version, a.updateBaseUrl, "arduinoAppsLabs/Stable")
	if err != nil {
		return fmt.Errorf("Error checking for updates: %w", err)
	}
	fmt.Println("Update available. Restart with ", restartPath)
	if restartPath != "" {
		return a.restartWithPath(restartPath)
	}
	return nil
}

type availableUpdateInfo struct {
	Version string
	Sha256  []byte
}

func (a *App) GetLatestVersion() (string, error) {
	env := runtime.Environment(a.ctx)
	fmt.Println("Arch:", env.Arch, "Platform:", env.Platform, "Type:", env.BuildType)
	plat := env.Platform + "-" + env.Arch

	// TODO: more robust way to generate the URL
	infoURL := a.updateBaseUrl + "arduinoAppsLabs/Stable/" + plat + ".json"

	fmt.Println("Fetching update info from", infoURL)
	resp, err := http.Get(infoURL)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("bad http status from %s: %v", infoURL, resp.Status)
	}
	defer resp.Body.Close()

	var res availableUpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if len(res.Sha256) != sha256.Size {
		return "", errors.New("bad cmd hash in info")
	}
	fmt.Println("Latest version:", res.Version)
	return res.Version, nil
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
	return adbfs.SyncFS(adbfs.AdbFSWriter{adbfs.AdbFS{Base: boardPath}}, os.DirFS(localPath))
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

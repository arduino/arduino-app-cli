package appsync

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/arduino/arduino-app-cli/pkg/adb"
	"github.com/arduino/arduino-app-cli/pkg/adbfs"
)

type AppsSync struct {
	adb *adb.ADBConnection

	OnPull func(name string, tmp string)
	OnPush func(name string)

	boardAppPath string

	mx       sync.RWMutex
	syncApps map[string]string

	watcher *fsnotify.Watcher
	stop    chan struct{}
}

func New(adb *adb.ADBConnection, boardAppPath string) (*AppsSync, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	a := &AppsSync{
		adb: adb,

		OnPull: func(name string, tmp string) {},
		OnPush: func(name string) {},

		boardAppPath: boardAppPath,

		mx:       sync.RWMutex{},
		syncApps: make(map[string]string),

		watcher: watcher,
		stop:    make(chan struct{}),
	}

	go a.watchLoop()

	return a, err
}

func (a *AppsSync) EnableSyncApp(name string) (string, error) {
	a.mx.Lock()
	defer a.mx.Unlock()
	if _, ok := a.syncApps[name]; ok {
		return "", fmt.Errorf("app %q is already synced", name)
	}

	remote := path.Join(a.boardAppPath, name)
	tmp, err := os.MkdirTemp("", "arduino-apps-sync_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	tmp = filepath.Join(tmp, name)
	err = os.MkdirAll(tmp, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// pull app from the remote
	if err := adbfs.SyncFS(adbfs.OsFSWriter{Base: tmp}, adbfs.NewAdbFS(remote, a.adb), ".cache"); err != nil {
		return "", fmt.Errorf("failed to pull app %q: %w", name, err)
	}
	a.OnPull(name, tmp)

	a.syncApps[name] = tmp

	// Add a path.
	err = fs.WalkDir(os.DirFS(tmp), ".", func(p string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			err := a.watcher.Add(filepath.Join(tmp, p))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to add watcher: %w", err)
	}

	return tmp, nil
}

func (a *AppsSync) DisableSyncApp(name string) error {
	// remove app from sync if is synced
	a.mx.Lock()
	defer a.mx.Unlock()
	tmp, ok := a.syncApps[name]
	if !ok {
		return fmt.Errorf("app %q is not synced", name)
	}
	delete(a.syncApps, name)

	// remove watcher from all subdirs
	err := fs.WalkDir(os.DirFS(tmp), ".", func(p string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			err := a.watcher.Remove(filepath.Join(tmp, p))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to remove watcher: %w", err)
	}

	// force push the last time
	if err = a.pushPath(tmp, path.Join(a.boardAppPath, name)); err != nil {
		return err
	}

	// remove temp dir
	os.RemoveAll(tmp)
	return nil
}

func (a *AppsSync) ForsePush(name string) error {
	// get synked app
	a.mx.RLock()
	defer a.mx.RUnlock()
	tmp, ok := a.syncApps[name]
	if !ok {
		return fmt.Errorf("app %q is not synced", name)
	}

	if err := a.pushPath(tmp, name); err != nil {
		return fmt.Errorf("failed to push app %q: %w", name, err)
	}

	return nil
}

func (a *AppsSync) pushPath(tmp string, name string) error {
	remote := path.Join(a.boardAppPath, name)
	err := adbfs.SyncFS(
		adbfs.NewAdbFS(remote, a.adb).ToWriter(),
		os.DirFS(tmp),
		".cache",
	)
	if err != nil {
		return err
	}
	a.OnPush(name)
	return nil
}

func (a *AppsSync) Close() {
	a.watcher.Close()
	<-a.stop
}

func (a *AppsSync) watchLoop() {
	defer func() {
		a.stop <- struct{}{}
	}()
	for {
		select {
		case event, ok := <-a.watcher.Events:
			if !ok {
				return
			}
			slog.Debug("watcher event", "op", event.Op, "path", event.Name)
			go func() {
				a.mx.RLock()
				defer a.mx.RUnlock()
				for name, tmp := range a.syncApps {
					if strings.HasPrefix(event.Name, tmp) {
						// TODO: we could improve this by do not push all files
						if err := a.pushPath(tmp, name); err != nil {
							slog.Warn("failed to push app", "app", name, "err", err)
						}
						break
					}
				}
			}()
			// If we get a create folder we need to add a watcher on that dir.
			if event.Has(fsnotify.Create) {
				if i, err := os.Stat(event.Name); err == nil && i.IsDir() {
					if err := a.watcher.Add(event.Name); err != nil {
						slog.Warn("failed to add watcher", "path", event.Name, "err", err)
					}
				}
			}
		case err := <-a.watcher.Errors:
			if err != nil {
				panic(err)
			}
		}
	}
}

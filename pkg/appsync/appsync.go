package appsync

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remotefs"
)

type AppsSync struct {
	conn remote.RemoteConn

	OnPull func(name string, tmp string)
	OnPush func(name string)

	mx       sync.RWMutex
	syncApps map[string]string

	watcher *fsnotify.Watcher
	stop    chan struct{}
}

var ignoredFiles = []string{".cache", "app.yml", "app.yaml"}

func New(conn remote.RemoteConn) (*AppsSync, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	a := &AppsSync{
		conn: conn,

		OnPull: func(name string, tmp string) {},
		OnPush: func(name string) {},

		mx:       sync.RWMutex{},
		syncApps: make(map[string]string),

		watcher: watcher,
		stop:    make(chan struct{}),
	}

	go a.watchLoop()

	return a, err
}

func (a *AppsSync) EnableSyncApp(path string) (string, error) {
	a.mx.Lock()
	defer a.mx.Unlock()
	if _, ok := a.syncApps[path]; ok {
		return "", fmt.Errorf("app %q is already synced", path)
	}

	tmp, err := os.MkdirTemp("", "arduino-apps-sync_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	tmp = filepath.Join(tmp, path)
	err = os.MkdirAll(tmp, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// pull app from the remote
	if err := SyncFS(OsFSWriter{Base: tmp}, remotefs.New(path, a.conn), ignoredFiles...); err != nil {
		return "", fmt.Errorf("failed to pull app %q: %w", path, err)
	}
	a.OnPull(path, tmp)

	a.syncApps[path] = tmp

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

func (a *AppsSync) DisableSyncApp(path string) error {
	// remove app from sync if is synced
	a.mx.Lock()
	defer a.mx.Unlock()
	tmp, ok := a.syncApps[path]
	if !ok {
		return fmt.Errorf("app %q is not synced", path)
	}
	delete(a.syncApps, path)

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
	if err = a.pushPath(tmp, path); err != nil {
		return err
	}

	// remove temp dir
	os.RemoveAll(tmp)
	return nil
}

func (a *AppsSync) ForsePush(path string) error {
	// get synked app
	a.mx.RLock()
	defer a.mx.RUnlock()
	tmp, ok := a.syncApps[path]
	if !ok {
		return fmt.Errorf("app %q is not synced", path)
	}

	if err := a.pushPath(tmp, path); err != nil {
		return fmt.Errorf("failed to push app %q: %w", path, err)
	}

	return nil
}

func (a *AppsSync) pushPath(tmp string, path string) error {
	err := SyncFS(
		remotefs.New(path, a.conn).ToWriter(),
		os.DirFS(tmp),
		ignoredFiles...,
	)
	if err != nil {
		return err
	}
	a.OnPush(path)
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

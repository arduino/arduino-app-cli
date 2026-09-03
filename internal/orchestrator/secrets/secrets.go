// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package secrets stores the value of the variables a brick declares secret, outside
// the app folder: a release is installed read-only and its app.yaml carries no value.
// A secret is one file, the shape docker compose mounts as /run/secrets/<name>.
package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"regexp"
	"slices"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// Only the user running the cli reads a secret.
const (
	fileMode = 0600
	dirMode  = 0700
)

type Store struct {
	dir *paths.Path
}

func NewStore(cfg config.Configuration) *Store {
	return &Store{dir: cfg.DataDir().Join("secrets")}
}

// Get is the values of an app. An app with no secrets stored is not an error.
func (s *Store) Get(id appid.ID) (map[string]string, error) {
	files, err := s.appDir(id).ReadDir()
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	for _, file := range files {
		if file.IsDir() || !nameIsValid(file.Base()) {
			slog.Warn("skipping a file that is not a secret", slog.String("path", file.String()))
			continue
		}
		// The mode comes from the open file, so it is the mode of what the read returns.
		value, err := func() ([]byte, error) {
			f, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer f.Close()

			info, err := f.Stat()
			if err != nil {
				return nil, err
			}
			// A mode the store did not write means the value is exposed already.
			if info.Mode().Perm() != fileMode {
				slog.Warn("skipping an exposed secret",
					slog.String("path", file.String()),
					slog.String("mode", fmt.Sprintf("%#o", info.Mode().Perm())))
				return nil, nil
			}
			return io.ReadAll(f)
		}()
		if err != nil {
			return nil, err
		}
		if value == nil {
			continue
		}
		values[file.Base()] = string(value)
	}
	return values, nil
}

// Set writes the updates of an app, one file each. A name set to an empty value is
// removed, and an app left with nothing has no directory.
func (s *Store) Set(id appid.ID, updates map[string]string) error {
	for name := range updates {
		if !nameIsValid(name) {
			return fmt.Errorf("%q is not a valid secret name", name)
		}
	}

	dir := s.appDir(id)
	// MkdirAll of go-paths-helper is 0755, and a secret is only for this user.
	if err := os.MkdirAll(dir.String(), dirMode); err != nil {
		return err
	}
	for name, value := range updates {
		file := dir.Join(name)
		if value == "" {
			if err := file.RemoveAll(); err != nil {
				return err
			}
			continue
		}
		if err := fatomic.WriteFile(file.String(), []byte(value), fileMode); err != nil {
			return err
		}
		// renameio keeps the mode a file already has, so the mode is set here.
		if err := file.Chmod(fileMode); err != nil {
			return err
		}
	}

	// The remove fails while the app has a secret left, which is the check we want.
	_ = dir.Remove()
	return nil
}

func (s *Store) Delete(id appid.ID) error {
	return s.appDir(id).RemoveAll()
}

// Move carries the values of an app to another id, as a rename or an upgrade does.
func (s *Store) Move(from, to appid.ID) error {
	values, err := s.Get(from)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}
	if err := s.Set(to, values); err != nil {
		return err
	}
	return s.Delete(from)
}

// Adopt moves the secrets an app keeps in its app.yaml into the store, and blanks the
// keys there. The api still writes app.yaml, so what it wrote always wins and is
// copied forward: an edit is never overruled by what the store already had.
func Adopt(store *Store, id appid.ID, arduinoApp *app.ArduinoApp, brickIndex *bricksindex.BricksIndex) error {
	brickIndex = brickIndex.WithAppBricks(arduinoApp.LocalBricks)

	found := map[string]string{}
	blanked := false
	for i := range arduinoApp.Descriptor.Bricks {
		brick := &arduinoApp.Descriptor.Bricks[i]
		brickDef, ok := brickIndex.FindBrickByID(brick.ID)
		if !ok {
			continue
		}
		for name, value := range brick.Variables {
			if variable, ok := brickDef.GetVariable(name); !ok || !variable.Secret || value == "" {
				continue
			}
			found[name] = value
			brick.Variables[name] = ""
			blanked = true
		}
	}
	if !blanked {
		return nil
	}

	if err := store.Set(id, found); err != nil {
		return err
	}

	// The value is safe now. An app.yaml the cli cannot write, an example or a
	// release, keeps its blanked copy for the next start to try again.
	if err := arduinoApp.Save(); err != nil {
		slog.Warn("cannot blank the adopted secrets in app.yaml",
			slog.String("app", id.String()),
			slog.Any("variables", slices.Sorted(maps.Keys(found))),
			slog.String("error", err.Error()))
	}
	return nil
}

// appDir is the directory of an app. The name is a hash, so that an app addressed by
// path does not overflow the file name limit, and is not in a directory listing.
func (s *Store) appDir(id appid.ID) *paths.Path {
	sum := sha256.Sum256([]byte(id.String()))
	return s.dir.Join(hex.EncodeToString(sum[:]))
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// nameIsValid is the shape of an environment variable name, the only thing a brick
// declares. The name is a file name here, so nothing else is written or read.
func nameIsValid(name string) bool {
	return envName.MatchString(name)
}

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package secrets stores the value of the variables a brick declares secret, outside
// the app folder: a release is installed read-only and its app.yaml carries no value.
package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"sync"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

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

	// A write is read-modify-write, and the daemon serves more than one request.
	mux sync.Mutex
}

func NewStore(cfg config.Configuration) *Store {
	return &Store{dir: cfg.DataDir().Join("secrets")}
}

// storeFile is one app. The id is written for whoever has to tell the files apart:
// the name is a hash, so that an app addressed by path does not overflow the file
// name limit, and does not put that path in a directory listing.
type storeFile struct {
	App     string            `yaml:"app"`
	Version int               `yaml:"version"`
	Values  map[string]string `yaml:"values"`
}

func (s *Store) path(id appid.ID) *paths.Path {
	sum := sha256.Sum256([]byte(id.String()))
	return s.dir.Join(hex.EncodeToString(sum[:]) + ".yaml")
}

// Get is the values of an app. An app with no secrets stored is not an error.
func (s *Store) Get(id appid.ID) (map[string]string, error) {
	file := s.path(id)
	data, err := file.ReadFile()
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	// The file is written 0600, but renameio keeps the mode a file already has.
	if info, err := file.Stat(); err == nil && info.Mode().Perm() != fileMode {
		slog.Warn("repairing the permissions of a secrets file", slog.String("path", file.String()))
		if err := os.Chmod(file.String(), fileMode); err != nil {
			return nil, err
		}
	}

	var stored storeFile
	if err := yaml.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", file, err)
	}
	if stored.Values == nil {
		stored.Values = map[string]string{}
	}
	return stored.Values, nil
}

// Set merges updates into what the app has. A name set to an empty value is removed,
// and an app left with nothing has no file.
func (s *Store) Set(id appid.ID, updates map[string]string) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	values, err := s.Get(id)
	if err != nil {
		return err
	}
	for name, value := range updates {
		if value == "" {
			delete(values, name)
			continue
		}
		values[name] = value
	}

	file := s.path(id)
	if len(values) == 0 {
		return file.RemoveAll()
	}

	data, err := yaml.Marshal(storeFile{App: id.String(), Version: 1, Values: values})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir.String(), dirMode); err != nil {
		return err
	}
	if err := fatomic.WriteFile(file.String(), data, fileMode); err != nil {
		return err
	}
	return os.Chmod(file.String(), fileMode)
}

func (s *Store) Delete(id appid.ID) error {
	return s.path(id).RemoveAll()
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

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package manifest reads the *downloaded.json files produced by the
// model-downloader container. Schema:
//
//	{"version": 1, "model_id": "...", "files": [{"path": "...", "size": N}, ...]}
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/arduino/go-paths-helper"
)

const FilenameSuffix = "downloaded.json"

type File struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type Manifest struct {
	Path    string
	Dir     string
	ModelID string
	Files   []File
}

func (m Manifest) Filename() string { return filepath.Base(m.Path) }

func (m Manifest) TotalSize() uint64 {
	var total uint64
	for _, f := range m.Files {
		if f.Size > 0 {
			total += uint64(f.Size)
		}
	}
	return total
}

// AbsPaths returns every artifact path plus the manifest file itself.
func (m Manifest) AbsPaths() []string {
	out := make([]string, 0, len(m.Files)+1)
	for _, f := range m.Files {
		out = append(out, filepath.Join(m.Dir, f.Path))
	}
	out = append(out, m.Path)
	return out
}

type rawManifest struct {
	Version int    `json:"version"`
	ModelID string `json:"model_id"`
	Files   []File `json:"files"`
}

func Read(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var parsed rawManifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(parsed.Files) == 0 {
		return Manifest{}, fmt.Errorf("%s: empty files list", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return Manifest{
		Path:    abs,
		Dir:     filepath.Dir(abs),
		ModelID: parsed.ModelID,
		Files:   parsed.Files,
	}, nil
}

// Verify returns nil iff every listed file exists at the recorded size.
func Verify(m Manifest) error {
	for _, f := range m.Files {
		abs := filepath.Join(m.Dir, f.Path)
		info, err := os.Stat(abs)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("missing file: %s", f.Path)
			}
			return fmt.Errorf("stat %s: %w", f.Path, err)
		}
		if info.IsDir() {
			return fmt.Errorf("expected file, found directory: %s", f.Path)
		}
		if info.Size() != f.Size {
			return fmt.Errorf("size mismatch for %s: expected %d, found %d", f.Path, f.Size, info.Size())
		}
	}
	return nil
}

// Find walks root recursively and returns every verified manifest.
// Unreadable or unverified manifests are dropped (broken/partial
// downloads must not be reported as installed). nil or missing root
// yields an empty slice.
func Find(root *paths.Path) []Manifest {
	if root == nil || root.NotExist() {
		return nil
	}
	var out []Manifest
	_ = filepath.WalkDir(root.String(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("manifest find: walk error", "path", path, "err", err)
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), FilenameSuffix) {
			return nil
		}
		m, err := Read(path)
		if err != nil {
			slog.Debug("manifest find: unreadable", "path", path, "err", err)
			return nil
		}
		if err := Verify(m); err != nil {
			slog.Debug("manifest find: verification failed", "path", path, "err", err)
			return nil
		}
		out = append(out, m)
		return nil
	})
	return out
}

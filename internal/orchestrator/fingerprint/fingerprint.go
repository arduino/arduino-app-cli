// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package fingerprint tells whether a start of an app would compile, flash and run
// anything new compared to its last start.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"slices"
	"sort"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

// fileName is where Save and Load keep the fingerprint, inside the app's cache dir.
const fileName = "fingerprint"

// fingerprintVersion is used to force compilation when bumped.
const fingerprintVersion = "1"

// Compute returns a stable identifier for the sketch, the board fqbn, whether it is
// flashed to ram, the platform core and the bricks an app start depends on. sketchPath
// and platforms can be left as nil/empty for an app with no sketch.
func Compute(sketchPath *paths.Path, fqbn string, flashToRAM bool, platforms string, bricks []app.Brick) (string, error) {
	h := sha256.New()

	if sketchPath != nil {
		if err := hashSketch(h, sketchPath); err != nil {
			return "", fmt.Errorf("failed to hash sketch: %w", err)
		}
	}

	fmt.Fprintf(h, "version=%s\n", fingerprintVersion)
	fmt.Fprintf(h, "fqbn=%s\n", fqbn)
	fmt.Fprintf(h, "flash_to_ram=%t\n", flashToRAM)
	fmt.Fprintf(h, "platforms=%s\n", platforms)
	fmt.Fprintf(h, "bricks=%s\n", bricksKey(bricks))

	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashSketch feeds every file below sketchPath into h, keyed by its relative path.
func hashSketch(h hash.Hash, sketchPath *paths.Path) error {
	files, err := sketchPath.ReadDirRecursive()
	if err != nil {
		return err
	}
	files.FilterOutDirs()
	files.Sort()

	for _, file := range files {
		rel, err := file.RelFrom(sketchPath)
		if err != nil {
			return err
		}
		content, err := file.ReadFile()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "file=%s\n", rel.String())
		h.Write(content)
		fmt.Fprintln(h)
	}
	return nil
}

// bricksKey canonicalizes bricks into an order-independent string.
func bricksKey(bricks []app.Brick) string {
	keys := make([]string, 0, len(bricks))
	for _, b := range bricks {
		vars := make([]string, 0, len(b.Variables))
		for name, value := range b.Variables {
			vars = append(vars, name+"="+value)
		}
		sort.Strings(vars)

		devices := slices.Clone(b.Devices)
		sort.Strings(devices)

		keys = append(keys, fmt.Sprintf("%s|%s|%s|%s", b.ID, b.Model, strings.Join(vars, ","), strings.Join(devices, ",")))
	}
	sort.Strings(keys)
	return strings.Join(keys, ";")
}

// Load returns the fingerprint saved for appPath, or "" if none was saved yet.
func Load(appPath *paths.Path) (string, error) {
	file := appPath.Join(".cache", fileName)
	if file.NotExist() {
		return "", nil
	}
	content, err := file.ReadFile()
	if err != nil {
		return "", fmt.Errorf("failed to read fingerprint: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

// Save persists the fingerprint of the app at appPath.
func Save(appPath *paths.Path, value string) error {
	dir := appPath.Join(".cache")
	if err := dir.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	return fatomic.WriteFile(dir.Join(fileName).String(), []byte(value), 0644)
}

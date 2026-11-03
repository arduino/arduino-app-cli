// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/arduino/go-paths-helper"
)

// sketchFingerprintFormat changes when what sketchInputs covers changes: every sketch is then
// built once more.
const sketchFingerprintFormat = "1"

// sketchInputs is what the sketch build depends on, as the bytes to hash: the content of every
// file of the sketch folder, sketch.yaml included, by relative path, the FQBN and the flash mode
// of the upload and the installed platforms with their versions. Never a modification time:
// the clock of the board lags after a boot.
func sketchInputs(sketchPath *paths.Path, fqbn string, flashToRAM bool, platforms string) ([]byte, error) {
	var inputs bytes.Buffer
	fmt.Fprintf(&inputs, "format=%s\nfqbn=%s\nflash_to_ram=%t\nplatforms=%s\n", sketchFingerprintFormat, fqbn, flashToRAM, platforms)
	files, err := sketchPath.ReadDirRecursive()
	if err != nil {
		return nil, fmt.Errorf("cannot read the sketch folder: %w", err)
	}
	files.FilterOutDirs()
	files.Sort()
	for _, file := range files {
		content, err := file.ReadFile()
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", file, err)
		}
		relative, err := file.RelFrom(sketchPath)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&inputs, "%s=%x\n", relative, sha256.Sum256(content))
	}
	return inputs.Bytes(), nil
}

// buildArtifacts are the files of the build folder the upload reads: the outputs named after
// the sketch, sketch.ino.elf and the binaries made from it.
func buildArtifacts(buildPath, sketchPath *paths.Path) []string {
	entries, err := buildPath.ReadDir()
	if err != nil {
		return nil
	}
	entries.FilterOutDirs()
	prefix := sketchPath.Base() + ".ino."
	artifacts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Base(), prefix) {
			artifacts = append(artifacts, entry.String())
		}
	}
	return artifacts
}

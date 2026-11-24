// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
)

// fingerprint is what a part of the app was last built from: a hash of the inputs of the
// build, saved in .cache/<part>/fingerprint.json next to its output once the build succeeded.
// A start whose inputs hash the same, and whose output is still usable, skips the build.
type fingerprint struct {
	path *paths.Path
}

type fingerprintRecord struct {
	Hash string `json:"hash"`
	// Files the output needs, each of which must still exist for the record to match.
	Files []string `json:"files,omitempty"`
}

func partFingerprint(cacheDir *paths.Path, part string) fingerprint {
	return fingerprint{path: cacheDir.Join(part, "fingerprint.json")}
}

// matches tells whether the saved hash is hash and every file the record names exists: a
// missing or unreadable record matches nothing.
func (f fingerprint) matches(hash string) bool {
	data, err := f.path.ReadFile()
	if err != nil {
		return false
	}
	var record fingerprintRecord
	if json.Unmarshal(data, &record) != nil || record.Hash != hash {
		return false
	}
	for _, file := range record.Files {
		if !paths.New(file).Exist() {
			return false
		}
	}
	return true
}

// save records hash and the files the output needs for the next start, in one step: no start
// ever reads a partial record.
func (f fingerprint) save(hash string, files ...string) error {
	if err := f.path.Parent().MkdirAll(); err != nil {
		return fmt.Errorf("failed to create the fingerprint folder: %w", err)
	}
	data, err := json.Marshal(fingerprintRecord{Hash: hash, Files: files})
	if err != nil {
		return err
	}
	return fatomic.WriteFile(f.path.String(), data, 0o644)
}

// hashOf is the fingerprint hash of the inputs of a build, given as bytes.
func hashOf(inputs []byte) string {
	sum := sha256.Sum256(inputs)
	return hex.EncodeToString(sum[:])
}

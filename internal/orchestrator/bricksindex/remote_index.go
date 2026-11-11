// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"time"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/downloader/v3"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

const RemoteIndexFileName = "brick-index.json"

var ErrIndexNotFound = errors.New("bricks index not found, run 'arduino-app-cli brick update-index' to download it")

// The namespace and the name of a brick become folder names and Python module names.
var validBrickIDPart = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

type RemoteIndex struct {
	// Bricks has an entry for each release of each brick.
	Bricks []RemoteBrick `json:"bricks"`
}

// RemoteBrick is a release of a brick in the bricks index.
type RemoteBrick struct {
	ID              string          `json:"id"`
	Namespace       string          `json:"namespace"`
	Name            string          `json:"name"`
	Version         *semver.Version `json:"version"`
	Description     string          `json:"description"`
	Deprecated      bool            `json:"deprecated"`
	DeprecationNote string          `json:"deprecation_note,omitempty"`
	URL             string          `json:"url"`
	// Hash is the SHA-256 of the archive, in hex.
	Hash       string `json:"hash"`
	Size       int64  `json:"size"`
	Repository string `json:"repository"`
	// SupportedBoards is empty when the release supports every board.
	SupportedBoards []string `json:"supported_boards,omitempty"`
}

// SupportsBoard reports whether the release can be used on the board of platform.
func (b RemoteBrick) SupportsBoard(platform platform.Platform) bool {
	return supportsBoard(b.SupportedBoards, platform)
}

// LoadRemoteIndex reads the index saved in dir. It returns ErrIndexNotFound if it was never downloaded.
func LoadRemoteIndex(dir *paths.Path) (*RemoteIndex, error) {
	data, err := dir.Join(RemoteIndexFileName).ReadFile()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrIndexNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading bricks index: %w", err)
	}
	return parseRemoteIndex(data)
}

func parseRemoteIndex(data []byte) (*RemoteIndex, error) {
	var index RemoteIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("decoding bricks index: %w", err)
	}
	if err := index.validate(); err != nil {
		return nil, fmt.Errorf("invalid bricks index: %w", err)
	}
	return &index, nil
}

func (idx *RemoteIndex) validate() error {
	seen := make(map[string][]*semver.Version, len(idx.Bricks))
	for _, b := range idx.Bricks {
		if err := b.validate(); err != nil {
			return err
		}
		for _, v := range seen[b.ID] {
			if v.Equal(b.Version) {
				return fmt.Errorf("duplicate release %s@%s", b.ID, b.Version)
			}
		}
		seen[b.ID] = append(seen[b.ID], b.Version)
	}
	return nil
}

func (b RemoteBrick) validate() error {
	namespace, _, err := splitBrickID(b.ID)
	if err != nil {
		return err
	}
	if namespace != b.Namespace {
		return fmt.Errorf("brick %q: namespace %q does not match its id", b.ID, b.Namespace)
	}
	// semver.Parse accepts an empty string.
	if b.Version == nil || b.Version.String() == "" {
		return fmt.Errorf("brick %q: missing version", b.ID)
	}
	ref := b.ID + "@" + b.Version.String()
	if b.Name == "" {
		return fmt.Errorf("brick %s: missing name", ref)
	}
	if u, err := url.Parse(b.URL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("brick %s: invalid url %q", ref, b.URL)
	}
	if hash, err := hex.DecodeString(b.Hash); err != nil || len(hash) != sha256.Size {
		return fmt.Errorf("brick %s: invalid SHA-256 hash %q", ref, b.Hash)
	}
	if b.Size <= 0 {
		return fmt.Errorf("brick %s: invalid size %d", ref, b.Size)
	}
	return nil
}

// RemoteIndexIsRecent returns true if the index saved in dir was updated less than maxAge ago.
func RemoteIndexIsRecent(dir *paths.Path, maxAge time.Duration) bool {
	info, err := dir.Join(RemoteIndexFileName).Stat()
	return err == nil && time.Since(info.ModTime()) < maxAge
}

// UpdateRemoteIndex downloads the index from indexURL and saves it in dir.
func UpdateRemoteIndex(ctx context.Context, config downloader.Config, indexURL *url.URL, dir *paths.Path) (*RemoteIndex, error) {
	indexPath := dir.Join(RemoteIndexFileName)
	if indexURL == nil {
		return nil, errors.New("bricks index URL not configured, set ARDUINO_APP_CLI__BRICKS_INDEX_URL")
	}
	if err := dir.MkdirAll(); err != nil {
		return nil, fmt.Errorf("creating bricks index directory: %w", err)
	}
	// Each update downloads to its own file, so concurrent updates do not need a lock.
	tempFile, err := paths.WriteToTempFile(nil, dir, "."+RemoteIndexFileName+"-")
	if err != nil {
		return nil, fmt.Errorf("creating temporary file: %w", err)
	}
	defer func() { _ = tempFile.Remove() }()

	if err := downloadFile(ctx, config, indexURL.String(), tempFile.String(), 0, 0, nil); err != nil {
		return nil, fmt.Errorf("updating bricks index: %w", err)
	}
	data, err := tempFile.ReadFile()
	if err != nil {
		return nil, fmt.Errorf("updating bricks index: %w", err)
	}

	// TODO: verify the signature of the index before trusting it.
	index, err := parseRemoteIndex(data)
	if err != nil {
		return nil, err
	}
	if err := fatomic.WriteFile(indexPath.String(), data, 0o644); err != nil {
		return nil, fmt.Errorf("saving bricks index: %w", err)
	}
	return index, nil
}

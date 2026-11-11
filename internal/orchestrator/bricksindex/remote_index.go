// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/gofrs/flock"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
)

const (
	RemoteIndexFileName     = "brick-index.json"
	remoteIndexEtagFileName = RemoteIndexFileName + ".etag"
	remoteIndexLockFileName = RemoteIndexFileName + ".lock"
)

var ErrIndexNotFound = errors.New("bricks index not found, run 'arduino-app-cli brick update-index' to download it")

type RemoteIndex struct {
	SourceCommit string        `json:"source_commit,omitempty"`
	Bricks       []RemoteBrick `json:"bricks"`
}

type RemoteBrick struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Namespace       string          `json:"namespace"`
	Deprecated      bool            `json:"deprecated"`
	DeprecationNote string          `json:"deprecation_note,omitempty"`
	Latest          string          `json:"latest"`
	Releases        []RemoteRelease `json:"releases"`
}

type RemoteRelease struct {
	Version     string `json:"version"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Hash        string `json:"hash"`
	Size        int64  `json:"size"`
	Repository  string `json:"repository"`
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
	seen := make(map[string]struct{}, len(idx.Bricks))
	for _, b := range idx.Bricks {
		namespace, name, err := ParseBrickID(b.ID)
		if err != nil || namespace == "" || name == "" {
			return fmt.Errorf("invalid brick id %q", b.ID)
		}
		if namespace != b.Namespace {
			return fmt.Errorf("brick %q: namespace %q does not match its id", b.ID, b.Namespace)
		}
		if _, ok := seen[b.ID]; ok {
			return fmt.Errorf("duplicate brick id %q", b.ID)
		}
		seen[b.ID] = struct{}{}
	}
	return nil
}

type UpdateRemoteIndexResult struct {
	// Updated is false when the server reported that the local index is already up to date.
	Updated bool
	Index   *RemoteIndex
}

// UpdateRemoteIndex downloads the index from indexURL and saves it in dir. The previous index is
// replaced only if the new one is valid.
func UpdateRemoteIndex(ctx context.Context, client *http.Client, indexURL *url.URL, dir *paths.Path, userAgent string) (UpdateRemoteIndexResult, error) {
	if indexURL == nil {
		return UpdateRemoteIndexResult{}, errors.New("bricks index URL not configured, set ARDUINO_APP_CLI__BRICKS_INDEX_URL")
	}
	if err := dir.MkdirAll(); err != nil {
		return UpdateRemoteIndexResult{}, fmt.Errorf("creating bricks index directory: %w", err)
	}

	// The lock keeps the index and its etag consistent when two updates run together.
	lock := flock.New(dir.Join(remoteIndexLockFileName).String())
	if _, err := lock.TryLockContext(ctx, 100*time.Millisecond); err != nil {
		return UpdateRemoteIndexResult{}, fmt.Errorf("locking bricks index: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	etagPath := dir.Join(remoteIndexEtagFileName)

	// Ask for a conditional download only if there is a valid index to fall back to.
	var etag string
	current, err := LoadRemoteIndex(dir)
	if err == nil {
		if data, err := etagPath.ReadFile(); err == nil {
			etag = strings.TrimSpace(string(data))
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL.String(), nil)
	if err != nil {
		return UpdateRemoteIndexResult{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := client.Do(req)
	if err != nil {
		return UpdateRemoteIndexResult{}, fmt.Errorf("downloading bricks index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && etag != "" {
		return UpdateRemoteIndexResult{Updated: false, Index: current}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return UpdateRemoteIndexResult{}, fmt.Errorf("downloading bricks index from %s: unexpected status %s", indexURL, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return UpdateRemoteIndexResult{}, fmt.Errorf("downloading bricks index: %w", err)
	}

	// TODO: verify the signature of the index before trusting it.
	index, err := parseRemoteIndex(data)
	if err != nil {
		return UpdateRemoteIndexResult{}, err
	}

	if err := fatomic.WriteFile(dir.Join(RemoteIndexFileName).String(), data, 0o644); err != nil {
		return UpdateRemoteIndexResult{}, fmt.Errorf("saving bricks index: %w", err)
	}

	// A stale etag would make the next update skip a changed index: drop it when it can't be refreshed.
	newEtag := resp.Header.Get("ETag")
	if newEtag == "" {
		_ = etagPath.RemoveAll()
	} else if err := fatomic.WriteFile(etagPath.String(), []byte(newEtag), 0o644); err != nil {
		slog.Warn("cannot save bricks index etag", "error", err)
		_ = etagPath.RemoveAll()
	}

	return UpdateRemoteIndexResult{Updated: true, Index: index}, nil
}

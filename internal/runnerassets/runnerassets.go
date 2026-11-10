// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package runnerassets downloads the python runner's static assets (docs, examples,
// compose templates, ...) bundled in the arduino/app-bricks-py release wheel, the same
// ones `task download-runner-assets` fetches with the `gh` CLI. This fetches the wheel
// straight from its release download URL, so it needs no external tool besides a
// network connection.
package runnerassets

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/arduino/go-paths-helper"
)

const (
	owner = "arduino"
	repo  = "app-bricks-py"

	// staticPrefix is where the wheel ships the assets, relative to its root.
	staticPrefix = "arduino/app_bricks/static/"
)

// Downloader fetches the static assets of a runner version from GitHub
type Downloader struct {
	httpClient *http.Client
}

// NewDownloader returns a ready-to-use Downloader.
func NewDownloader() *Downloader {
	return &Downloader{httpClient: http.DefaultClient}
}

// Download fetches the wheel of the given runner version and
// extracts its static assets into dest/<version>, wiping that version's own directory
func (d *Downloader) Download(ctx context.Context, version string, dest *paths.Path) error {
	wheelName := fmt.Sprintf("arduino_app_bricks-%s-py3-none-any.whl", version)
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, pathEscapeTag("bricks/"+version), wheelName)

	wheel, err := d.get(ctx, url)
	if err != nil {
		return fmt.Errorf("cannot download %s: %w", wheelName, err)
	}

	versionDir := dest.Join(version)
	if err := versionDir.RemoveAll(); err != nil {
		return fmt.Errorf("cannot clean up %s: %w", versionDir, err)
	}
	if err := extractStatic(wheel, versionDir); err != nil {
		return fmt.Errorf("cannot extract %s: %w", wheelName, err)
	}
	return nil
}

// pathEscapeTag percent-encodes the tag's slash as %2F, so "bricks/0.13.2" lands in the
// URL as the single path segment GitHub expects for a tag, not as two segments naming a
// release "bricks" followed by a stray "0.13.2" path component.
func pathEscapeTag(tag string) string {
	return strings.ReplaceAll(tag, "/", "%2F")
}

// get issues a GET and returns the response body, following GitHub's redirect from the
// release download URL to the asset's actual content.
func (d *Downloader) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read the response of %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s: %s", url, resp.Status, body)
	}
	return body, nil
}

// extractStatic unzips the arduino/app_bricks/static/ folder of the wheel into dest.
func extractStatic(wheel []byte, dest *paths.Path) error {
	zipReader, err := zip.NewReader(bytes.NewReader(wheel), int64(len(wheel)))
	if err != nil {
		return fmt.Errorf("not a valid wheel: %w", err)
	}

	var extracted int
	for _, file := range zipReader.File {
		relPath, ok := strings.CutPrefix(file.Name, staticPrefix)
		if !ok || relPath == "" {
			continue
		}

		target := dest.Join(relPath)
		if file.FileInfo().IsDir() {
			if err := target.MkdirAll(); err != nil {
				return err
			}
			continue
		}

		if err := target.Parent().MkdirAll(); err != nil {
			return err
		}
		if err := copyZipFile(file, target); err != nil {
			return err
		}
		extracted++
	}

	if extracted == 0 {
		return fmt.Errorf("the wheel has no %s folder", staticPrefix)
	}
	return nil
}

// maxExtractedFileSize for assets
const maxExtractedFileSize = 5 << 20 // 5MiB

func copyZipFile(file *zip.File, target *paths.Path) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.OpenFile(target.String(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	// The +1 lets written exceed maxExtractedFileSize by one, so going over it is
	// detected instead of silently truncating the file at the limit.
	written, err := io.Copy(out, io.LimitReader(src, maxExtractedFileSize+1))
	if err != nil {
		return err
	}
	if written > maxExtractedFileSize {
		return fmt.Errorf("%s is larger than the %d bytes limit", file.Name, maxExtractedFileSize)
	}
	return out.Close()
}

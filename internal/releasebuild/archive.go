// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package releasebuild

import (
	"fmt"
	"io"

	"github.com/arduino/go-paths-helper"
)

// ResolveArchivePath resolves where a release archive goes, without creating it. fileName
// names it when output is a directory or is left out; an output file is taken as it is.
func ResolveArchivePath(fileName string, output *paths.Path, overwrite bool) (*paths.Path, error) {
	archivePath := paths.New(fileName)
	if output != nil {
		archivePath = output
		if archivePath.IsDir() {
			archivePath = archivePath.Join(fileName)
		}
	}
	archivePath, err := archivePath.Abs()
	if err != nil {
		return nil, err
	}

	if archivePath.Exist() && !overwrite {
		return nil, fmt.Errorf("%s already exists", archivePath)
	}
	return archivePath, nil
}

// WriteArchive writes the release stream to the file at path, creating it and copying
// until the stream ends.
func WriteArchive(reader io.Reader, path *paths.Path) error {
	file, err := path.Create()
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package board

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

const R0_IMAGE_VERSION_ID = "20250807-136"

// GetOSImageVersion returns the version of the OS image used in the board.
// It is used by the AppLab to enforce image version compatibility.
func GetOSImageVersion(rfs remote.FS) (string, error) {
	f, err := rfs.ReadFile("/etc/buildinfo")
	// The first R0 image has no buildinfo file.
	if errors.Is(err, fs.ErrNotExist) {
		return R0_IMAGE_VERSION_ID, nil
	}
	if err != nil {
		return "", fmt.Errorf("unable to read the buildinfo file: %w", err)
	}
	defer f.Close()

	if version, ok := parseOSImageVersion(f); ok {
		return version, nil
	}

	return "", fmt.Errorf("unable to find OS Image version in buildinfo file")
}

func parseOSImageVersion(r io.Reader) (string, bool) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		key, value, ok := strings.Cut(line, "=")
		if !ok || key != "BUILD_ID" {
			continue
		}

		version := strings.TrimSpace(value)
		if version != "" {
			return version, true
		}
	}

	if err := scanner.Err(); err != nil {
		return "", false
	}

	return "", false
}

// Calculates whether user partition preservation is supported,
// according to the current and target OS image versions.
//
// Preservation is supported if both versions are not the R0 image.
func IsUserPartitionPreservationSupported(currentImageVersion string, targetImageVersion string) bool {
	return targetImageVersion != R0_IMAGE_VERSION_ID && currentImageVersion != R0_IMAGE_VERSION_ID
}

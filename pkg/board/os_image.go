// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package board

import (
	"bufio"
	"io"
	"log/slog"
	"strings"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

const R0_IMAGE_VERSION_ID = "20250807-136"

// GetOSImageVersion returns the version of the OS image used in the board.
// It is used by the AppLab to enforce image version compatibility.
func GetOSImageVersion(rfs remote.FS) string {
	f, err := rfs.ReadFile("/etc/buildinfo")
	if err != nil {
		slog.Warn("Unable to read buildinfo file", "err", err, "using_default", R0_IMAGE_VERSION_ID)
		return R0_IMAGE_VERSION_ID
	}
	defer f.Close()

	if version, ok := parseOSImageVersion(f); ok {
		return version
	}
	slog.Warn("Unable to find OS Image version", "using_default", R0_IMAGE_VERSION_ID)
	return R0_IMAGE_VERSION_ID
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

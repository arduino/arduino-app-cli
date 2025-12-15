// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package flasherapi

import (
	"context"
	"log/slog"
	"strings"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

const R0_IMAGE_VERSION_ID = "20250807-136"

// GetOSImageVersion returns the version of the OS image used in the board.
// It is used by the AppLab to enforce image version compatibility.
func GetOSImageVersion(ctx context.Context, conn remote.RemoteConn) (string, error) {

	output, err := conn.GetCmd("cat", "/etc/buildinfo").Output(ctx)
	if err != nil {
		return R0_IMAGE_VERSION_ID, err
	}

	if version, ok := ParseOSImageVersion(string(output)); ok {
		slog.Info("find OS Image version", "version", version)
		return version, nil
	}
	slog.Info("Unable to find OS Image version", "using default version", R0_IMAGE_VERSION_ID)
	return R0_IMAGE_VERSION_ID, nil
}

func ParseOSImageVersion(buildInfo string) (string, bool) {
	for _, line := range strings.Split(buildInfo, "\n") {
		line = strings.TrimSpace(line)

		key, value, ok := strings.Cut(line, "=")
		if !ok || key != "BUILD_ID" {
			continue
		}

		version := strings.Trim(value, "\"' ")
		if version != "" {
			return version, true
		}
	}
	return "", false
}

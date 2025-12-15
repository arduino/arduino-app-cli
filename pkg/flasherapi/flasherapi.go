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

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

// GetOSImageVersion returns the version of the OS image used in the board.
// It is used by the AppLab to enforce image version compatibility.
func GetOSImageVersion(ctx context.Context, conn remote.RemoteConn) string {
	// if no version is set, return a default value
	return "20251123-159"
}

type OSImageRelease struct {
	VersionLabel string
	ID           string
	Latest       bool
}

func ListAvailableOSImages() []OSImageRelease {
	return []OSImageRelease{
		{
			ID:           "20251123-159",
			VersionLabel: "r159 (2025-11-23)",
			Latest:       true,
		},
	}
}

const R0_IMAGE_VERSION_ID = "20250807-136"

// Calculates whether user partition preservation is supported,
// according to the current and target OS image versions.
//
// Preservation is supported if both versions are not the R0 image.
func IsUserPartitionPreservationSupported(ctx context.Context, conn remote.RemoteConn, targetImageVersion OSImageRelease) bool {
	currentImageVersion := GetOSImageVersion(ctx, conn)
	return !(targetImageVersion.ID == R0_IMAGE_VERSION_ID || currentImageVersion == R0_IMAGE_VERSION_ID)
}

type FlashStep string

const (
	FlashStepPrecheck    FlashStep = "precheck"
	FlashStepDetection   FlashStep = "detection"
	FlashStepDownloading FlashStep = "downloading"
	FlashStepExtracting  FlashStep = "extracting"
	FlashStepFlashing    FlashStep = "flashing"
)

type FlashEvent struct {
	Step            FlashStep
	OverallProgress int    // percentage 0-100
	Message         string // log message to show on the UI
}

func Flash(
	ctx context.Context, // context to cancel to interrupt the flashing process
	// conn remote.RemoteConn, // is this required for the flash process?
	imageVersion OSImageRelease, // OS image version to flash
	preserveUserPartition bool, // whether to preserve the user partition or not
	eventCB func(event FlashEvent), // Callback, sends progress events to the caller
) error {
	// The disk space check is done before starting the flashing process.
	return InsufficientDiskSpaceError{
		RequiredSpaceMB:  2048,
		AvailableSpaceMB: 1024,
	}
}

// Errors returned by the Flash function above.
// Assertions can be done on the caller side to show better error messages.

type QDLFlashError struct {
	Details string
	Cause   error
}

type DownloadError struct {
	Details string
}

type InsufficientDiskSpaceError struct {
	RequiredSpaceMB  int64
	AvailableSpaceMB int64
}

func (e InsufficientDiskSpaceError) Error() string {
	return "insufficient disk space"
}

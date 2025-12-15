package flasherapi

import (
	"context"
	"log/slog"
	"strings"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

// GetOSImageVersion returns the version of the OS image used in the board.
// It is used by the AppLab to enforce image version compatibility.
func GetOSImageVersion(ctx context.Context, conn remote.RemoteConn) (string, error) {
	const defaultVersion = "20250807-136"

	output, err := conn.GetCmd("cat /etc/buildinfo").Output(ctx)
	if err != nil {
		return defaultVersion, err
	}

	if version, ok := ParseOSImageVersion(string(output)); ok {
		slog.Info("find OS Image version", "version", version)
		return version, nil
	}
	slog.Info("Unable to find OS Image version", "using default version", defaultVersion)
	return defaultVersion, nil
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

func IsUserPartitionPreservationSupported(conn remote.RemoteConn, targetImageVersion OSImageRelease) bool {
	// targetImageVersion is the version of the image to be flashed
	// some older versions do not support user partition preservation
	// so this has to be considered here
	return true
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

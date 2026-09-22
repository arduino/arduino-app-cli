// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package arduino

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/update"
)

func TestUpgradePackagesRejectsInvalidTargets(t *testing.T) {
	tests := []struct {
		name        string
		pkg         update.PackageInfo
		expectedErr string
	}{
		{
			name:        "Version outside the constraint",
			pkg:         update.PackageInfo{Name: "arduino:zephyr", ToVersion: "2.2.0"},
			expectedErr: "does not satisfy the version constraint",
		},
		{
			name:        "Version not parsable",
			pkg:         update.PackageInfo{Name: "arduino:zephyr", ToVersion: "not-a-version"},
			expectedErr: "invalid target version",
		},
		{
			name:        "Version empty",
			pkg:         update.PackageInfo{Name: "arduino:zephyr", ToVersion: ""},
			expectedErr: "target version is empty",
		},
		{
			name:        "Unsupported package",
			pkg:         update.PackageInfo{Name: "arduino:renesas", ToVersion: "0.5.0"},
			expectedErr: "unexpected package name",
		},
	}

	constraint, err := semver.ParseConstraint("<2.0.0")
	require.NoError(t, err, "Setup: failed to parse constraint")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := NewArduinoPlatformUpdater(platform.Platform{PlatformID: "arduino:zephyr"}, constraint)

			err := updater.UpgradePackages(context.Background(), []update.PackageInfo{tt.pkg}, func(update.Event) {
				t.Error("no event must be emitted for a rejected upgrade")
			})

			require.ErrorContains(t, err, tt.expectedErr)
		})
	}
}

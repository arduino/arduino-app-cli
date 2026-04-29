// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package board

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsurePlatformInstalled(t *testing.T) {
	// We skip it in CI, as downloading andinstalling the core takes ~6 minutes
	if os.Getenv("CI") != "" {
		t.Skip("Skipping slow test")
	}
	// Example test function
	err := EnsurePlatformInstalled(t.Context(), "arduino:zephyr:unoq")
	require.NoError(t, err)
}

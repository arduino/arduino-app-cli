// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package adb

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindAdbPath(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test which requires adb to be installed in CI environment")
	}

	got := FindAdbPath()
	if runtime.GOOS == "windows" {
		assert.True(t, filepath.Base(got) == "adb.exe")
	} else {
		assert.True(t, filepath.Base(got) == "adb")
	}
	assert.True(t, filepath.IsAbs(got))
	t.Logf("FindAdbPath returned: %q", got)
}

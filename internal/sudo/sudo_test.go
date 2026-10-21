// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package sudo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	for _, test := range []struct {
		name   string
		output string
		status string
	}{
		{name: "rules checked by username", output: "NEEDRESTART_MODE DEBIAN_FRONTEND !authenticate", status: "0"},
		{name: "missing permissions are reported", output: "NEEDRESTART_MODE DEBIAN_FRONTEND", status: "0"},
		{name: "lookup failure is reported", output: "sudo: unknown user arduino", status: "1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "calls")
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TEST_SUDO_LOG", logPath)
			t.Setenv("TEST_SUDO_OUTPUT", test.output)
			t.Setenv("TEST_SUDO_STATUS", test.status)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "sudo"), []byte(`#!/bin/sh
printf '%s\n' "$*" >> "$TEST_SUDO_LOG"
printf '%s\n' "$TEST_SUDO_OUTPUT"
exit "$TEST_SUDO_STATUS"
`), 0755))

			missing, err := Check(t.Context())
			if test.status != "0" {
				require.ErrorContains(t, err, "cannot list the sudo rules of arduino")
				require.ErrorContains(t, err, test.output)
			} else {
				require.NoError(t, err)
				if strings.Contains(test.output, "!authenticate") {
					require.Empty(t, missing)
				} else {
					require.Contains(t, missing, "apt-get update")
				}
			}
			calls, err := os.ReadFile(logPath)
			require.NoError(t, err)
			for call := range strings.SplitSeq(strings.TrimSpace(string(calls)), "\n") {
				require.True(t, strings.HasPrefix(call, "-n -l -l -U arduino"), call)
			}
		})
	}
}

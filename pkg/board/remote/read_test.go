// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote_test

import (
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

func TestCmdError(t *testing.T) {
	exitErr := errors.New("exit status 1")

	tests := []struct {
		name     string
		stderr   string
		expected error
	}{
		{"missing file", "cat: '/etc/nope': No such file or directory", fs.ErrNotExist},
		{"file command on missing path", "/etc/nope: cannot open `/etc/nope' (No such file or directory)", fs.ErrNotExist},
		{"no permission", "cat: /etc/shadow: Permission denied", fs.ErrPermission},
		{"device offline", "error: device offline", remote.ErrConnLost},
		{"device not found", "error: device '10.0.0.1:5555' not found", remote.ErrConnLost},
		{"connection dropped", "Connection closed by remote host", remote.ErrConnLost},
		{"unknown failure", "something else went wrong", exitErr},
		{"no stderr", "", exitErr},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := remote.CmdError(exitErr, []byte(tc.stderr))
			require.ErrorIs(t, err, tc.expected)
			if tc.stderr != "" {
				require.ErrorContains(t, err, tc.stderr)
			}
		})
	}

	t.Run("command succeeded", func(t *testing.T) {
		require.NoError(t, remote.CmdError(nil, []byte("cat: No such file or directory")))
	})
}

func TestPeekOutput(t *testing.T) {
	t.Run("command failed before the first byte", func(t *testing.T) {
		_, err := remote.PeekOutput(strings.NewReader(""), func() error { return fs.ErrNotExist })
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("empty file", func(t *testing.T) {
		r, err := remote.PeekOutput(strings.NewReader(""), func() error { return nil })
		require.NoError(t, err)
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Empty(t, data)
	})

	t.Run("the failure is not asked when the command writes", func(t *testing.T) {
		calls := 0
		r, err := remote.PeekOutput(strings.NewReader("Hello, World!"), func() error {
			calls++
			return nil
		})
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "Hello, World!", string(data))
		require.Equal(t, 0, calls)
	})
}

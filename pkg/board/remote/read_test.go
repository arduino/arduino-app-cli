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
	"testing/iotest"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

func TestReadError(t *testing.T) {
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
			err := remote.ReadError(exitErr, []byte(tc.stderr))
			require.ErrorIs(t, err, tc.expected)
			if tc.stderr != "" {
				require.ErrorContains(t, err, tc.stderr)
			}
		})
	}

	t.Run("command succeeded", func(t *testing.T) {
		require.NoError(t, remote.ReadError(nil, []byte("cat: No such file or directory")))
	})
}

func TestStartRead(t *testing.T) {
	t.Run("command failed before the first byte", func(t *testing.T) {
		calls := 0
		_, err := remote.StartRead(strings.NewReader(""), func(bool) error {
			calls++
			return fs.ErrNotExist
		})
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.Equal(t, 1, calls)
	})

	t.Run("empty file", func(t *testing.T) {
		r, err := remote.StartRead(strings.NewReader(""), func(bool) error { return nil })
		require.NoError(t, err)
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Empty(t, data)
		require.NoError(t, r.Close())
	})

	t.Run("read failed before the first byte", func(t *testing.T) {
		readErr := errors.New("pipe is broken")
		_, err := remote.StartRead(iotest.ErrReader(readErr), func(bool) error { return nil })
		require.ErrorIs(t, err, readErr)
	})

	t.Run("command failed mid stream", func(t *testing.T) {
		r, err := remote.StartRead(strings.NewReader("partial"), func(bool) error { return remote.ErrConnLost })
		require.NoError(t, err)

		// The truncated content must not pass as a complete read.
		data, err := io.ReadAll(r)
		require.ErrorIs(t, err, remote.ErrConnLost)
		require.Equal(t, "partial", string(data))
		require.ErrorIs(t, r.Close(), remote.ErrConnLost)
	})

	t.Run("wait runs once", func(t *testing.T) {
		calls := 0
		r, err := remote.StartRead(strings.NewReader("Hello, World!"), func(bool) error {
			calls++
			return nil
		})
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "Hello, World!", string(data))
		require.NoError(t, r.Close())
		require.Equal(t, 1, calls)
	})
}

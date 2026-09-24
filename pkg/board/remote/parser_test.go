// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReadOutput(t *testing.T) {
	exitErr := errors.New("exit status 1")

	tests := []struct {
		name     string
		stderr   string
		expected error
	}{
		{"missing file", "cat: '/etc/nope': No such file or directory", fs.ErrNotExist},
		{"no permission", "cat: /etc/shadow: Permission denied", fs.ErrPermission},
		{"unknown failure", "something else went wrong", exitErr},
		{"no stderr", "", exitErr},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseReadOutput(strings.NewReader(""), func() ([]byte, error) {
				return []byte(tc.stderr), exitErr
			})
			require.ErrorIs(t, err, tc.expected)
			if tc.stderr != "" {
				require.ErrorContains(t, err, tc.stderr)
			}
		})
	}

	t.Run("empty file", func(t *testing.T) {
		r, err := ParseReadOutput(&closedByWait{}, func() ([]byte, error) {
			// A read that succeeds can still write on stderr.
			return []byte("adb: warning"), nil
		})
		require.NoError(t, err)
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Empty(t, data)
	})

	t.Run("the command end is asked at the end of the file", func(t *testing.T) {
		calls := 0
		r, err := ParseReadOutput(strings.NewReader("Hello, World!"), func() ([]byte, error) {
			calls++
			return nil, nil
		})
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "Hello, World!", string(data))
		require.Equal(t, 1, calls)
	})

	t.Run("no output and no failure to explain it", func(t *testing.T) {
		_, err := ParseReadOutput(failingReader{}, func() ([]byte, error) { return nil, nil })
		require.ErrorIs(t, err, os.ErrClosed)
	})
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, os.ErrClosed }

// closedByWait is the output of an empty file: the wait closes the pipe, so
// only the first read reports the end.
type closedByWait struct{ reads int }

func (r *closedByWait) Read([]byte) (int, error) {
	r.reads++
	if r.reads > 1 {
		return 0, os.ErrClosed
	}
	return 0, io.EOF
}

func TestParseLsOutput(t *testing.T) {
	input := `total 20
drwxr-xr-x 2 u g 4096 Jan  1 12:00 "."
drwxr-xr-x 3 u g 4096 Jan  1 12:00 ".."
-rw-r--r-- 1 u g   13 Jan  1 12:00 "regular.txt"
drwxr-xr-x 2 u g 4096 Jan  1 12:00 "subdir"
lrwxrwxrwx 1 u g   10 Jan  1 12:00 "link" -> "target"
`
	got, err := ParseLsOutput(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, []FileInfo{
		{Name: "regular.txt"},
		{Name: "subdir", IsDir: true},
		{Name: "link", IsSymlink: true},
	}, got)
}

func TestParseChage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{
			name: "user set",
			input: `
Last password change                                    : Jun 30, 2025
Password expires                                        : never
Password inactive                                       : never
Account expires                                         : never
Minimum number of days between password change          : 0
Maximum number of days between password change          : 99999
Number of days of warning before password expires       : 7`,
			want: true,
		},
		{
			name: "user not set",
			input: `
Last password change                                    : password must be changed
Password expires                                        : password must be changed
Password inactive                                       : password must be changed
Account expires                                         : never
Minimum number of days between password change          : 0
Maximum number of days between password change          : 99999
Number of days of warning before password expires       : 7`,
			want: false,
		},
		{
			name: "malformed input",
			input: `
Last password change                                    - Jun 30, 2025
Password expires                                        : never
`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no relevant line",
			input:   "something else",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChage(strings.NewReader(tt.input))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

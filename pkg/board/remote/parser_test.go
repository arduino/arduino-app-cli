// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package composefile

import (
	"slices"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPorts(t *testing.T) {
	testCases := []struct {
		name      string
		content   string
		want      []string
		expectErr bool
	}{
		{
			name: "basic",
			content: `
version: "3"
services:
  web:
    ports:
      - "8080:80"
      - "443:443"
  db:
    ports:
      - "5432"
      - "127.0.0.1:15432:5432"
  cache:
    ports:
      - "6379"
      - "6380:6380"
  multi:
    ports:
      - "0.0.0.0:9000:9000/tcp"
      - "10000:10000"
`,
			want:      []string{"8080", "443", "5432", "15432", "6379", "6380", "9000", "10000"},
			expectErr: false,
		},
		{
			name: "no_ports",
			content: `
version: "3"
services:
  web:
    image: nginx
  db:
    image: postgres
`,
			want:      nil,
			expectErr: false,
		},
		{
			name: "invalid_yaml",
			content: `
version: "3"
services
  web:
    ports: [8080:80]
`,
			want:      nil,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := paths.New(t.TempDir()).Join("compose.yaml")
			err := tmpFile.WriteFile([]byte(tc.content))
			require.NoError(t, err)

			got, err := ExtractPorts(tmpFile)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				slices.Sort(tc.want)
				slices.Sort(got)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

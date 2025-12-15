// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package flasherapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOSImageVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		found    bool
	}{
		{
			name: "valid build id",
			input: `BUILD_ID="20251006-395"
			VARIANT_ID=xfce"
			`,
			expected: "20251006-395",
			found:    true,
		},
		{
			name:  "missing build id",
			input: "VARIANT_ID=xfce\n",
			found: false,
		},
		{
			name:  "empty build id",
			input: "BUILD_ID=\n",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseOSImageVersion(strings.NewReader(tt.input))
			require.Equal(t, tt.found, ok)
			require.Equal(t, tt.expected, got)
		})
	}
}

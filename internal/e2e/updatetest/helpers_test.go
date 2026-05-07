// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package updatetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenMinorTag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "decrement patch version",
			input:    "1.2.3",
			expected: "v1.2.2",
		},
		{
			name:     "patch is zero, decrement minor",
			input:    "1.2.0",
			expected: "v1.1.0",
		},
		{
			name:     "minor and patch are zero, decrement major",
			input:    "1.0.0",
			expected: "v0.9.0",
		},
		{
			name:     "with v prefix",
			input:    "v1.2.3",
			expected: "v1.2.2",
		},
		{
			name:     "major version 0, no decrement",
			input:    "0.0.0",
			expected: "v0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := genMinorTag(t, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

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

package properties

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateKey(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "valid simple key",
			input:       "key",
			expectError: false,
		},
		{
			name:        "valid key with numbers",
			input:       "test-key-1",
			expectError: false,
		},
		{
			name:        "valid key with dot and underscore",
			input:       "my_config.value",
			expectError: false,
		},
		{
			name:        "key at max length",
			input:       strings.Repeat("a", maxKeyLength),
			expectError: false,
		},
		{
			name:        "empty key",
			input:       "",
			expectError: true,
		},
		{
			name:        "key too long",
			input:       strings.Repeat("a", maxKeyLength+1),
			expectError: true,
		},
		{
			name:        "key with invalid space",
			input:       "my key",
			expectError: true,
		},
		{
			name:        "key with invalid symbols",
			input:       "test!",
			expectError: true,
		},
		{
			name:        "key with slashes",
			input:       "path/to/value",
			expectError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateKey(tc.input)

			if tc.expectError && err == nil {
				require.Error(t, err, "expected an error but got none")
			}
			if !tc.expectError && err != nil {
				require.NoError(t, err, "did not expect an error but got one: %v", err)
			}
		})
	}
}

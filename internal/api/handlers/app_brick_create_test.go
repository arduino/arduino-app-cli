// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateBrickID(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedID   string
		errorMessage string
	}{
		{
			name:       "simple name",
			input:      "MyBrick",
			expectedID: "mybrick",
		},
		{
			name:       "name with spaces",
			input:      "My Awesome Brick",
			expectedID: "my_awesome_brick",
		},
		{
			name:       "name with hyphens",
			input:      "my-brick",
			expectedID: "my_brick",
		},
		{
			name:       "name with numbers",
			input:      "Brick123",
			expectedID: "brick123",
		},
		{
			name:       "name with mixed case and special chars",
			input:      "My-Awesome_Brick",
			expectedID: "my_awesome_brick",
		},
		{
			name:       "name with leading/trailing spaces",
			input:      "  MyBrick  ",
			expectedID: "mybrick",
		},
		{
			name:       "single character name",
			input:      "A",
			expectedID: "a",
		},
		{
			name:         "only special characters",
			input:        "---___",
			expectedID:   "",
			errorMessage: "brick name must contain at least one alphanumeric character",
		},
		{
			name:       "mixed valid and underscore",
			input:      "my_brick",
			expectedID: "my_brick",
		},
		{
			name:       "converts to lowercase",
			input:      "MyBrick",
			expectedID: "mybrick",
		},
		{
			name:       "replaces spaces with underscores",
			input:      "my brick",
			expectedID: "my_brick",
		},
		{
			name:       "replaces multiple spaces with single underscore",
			input:      "my   brick",
			expectedID: "my_brick",
		},
		{
			name:       "replaces hyphens with underscores",
			input:      "my-brick",
			expectedID: "my_brick",
		},
		{
			name:       "trims leading underscores",
			input:      "___mybrick",
			expectedID: "mybrick",
		},
		{
			name:       "trims trailing underscores",
			input:      "mybrick___",
			expectedID: "mybrick",
		},
		{
			name:       "trims both sides",
			input:      "___mybrick___",
			expectedID: "mybrick",
		},
		{
			name:       "keeps internal alphanumerics and underscores",
			input:      "my_brick_123",
			expectedID: "my_brick_123",
		},
		{
			name:       "alphanumeric",
			input:      "abc123",
			expectedID: "abc123",
		},
		{
			name:       "uppercase",
			input:      "MYBRICK",
			expectedID: "mybrick",
		},
		{
			name:       "lowercase",
			input:      "mybrick",
			expectedID: "mybrick",
		},
		{
			name:       "numbers only",
			input:      "123456",
			expectedID: "123456",
		},
		{
			name:       "with special chars that get converted",
			input:      "my@brick#test",
			expectedID: "my_brick_test",
		},
		{
			name:         "name with dot character - should error",
			input:        "my.brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with multiple dots - should error",
			input:        "my.awesome.brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with dot at the beginning - should error",
			input:        ".mybrick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "complex valid name with dot",
			input:        "My Awesome Brick v2.0",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with colon character - should error",
			input:        "my:brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain ':' character",
		},
		{
			name:         "name with multiple colons - should error",
			input:        "my:awesome:brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain ':' character",
		},
		{
			name:         "name with colon at the end - should error",
			input:        "mybrick:",
			expectedID:   "",
			errorMessage: "brick name cannot contain ':' character",
		},
		{
			name:         "rejects dot before colon",
			input:        "test.name:brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with both dot and colon - should error on dot first",
			input:        "my.brick:test",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := generateBrickID(tt.input)

			if tt.errorMessage != "" {
				require.Error(t, err, "expected error but got none")
				require.Equal(t, tt.errorMessage, err.Error(), "error message mismatch")
			} else {
				require.NoError(t, err, "expected no error but got: %v", err)
				require.Equal(t, tt.expectedID, id, "generated ID mismatch")
			}
		})
	}
}

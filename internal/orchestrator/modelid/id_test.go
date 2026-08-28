// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelid

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in         string
		namespace  string
		name       string
		repository string
		fileName   string
	}{
		{
			// Two thirds of models-list.yaml is this shape.
			in: "face-detection", namespace: "", name: "face-detection",
			repository: "", fileName: "face-detection",
		},
		{
			in: "ei:efficientnet-b4", namespace: "ei", name: "efficientnet-b4",
			repository: "", fileName: "efficientnet-b4",
		},
		{
			in: "llamacpp:gemma-3-1b-it-Q4_0", namespace: "llamacpp", name: "gemma-3-1b-it-Q4_0",
			repository: "", fileName: "gemma-3-1b-it-Q4_0",
		},
		{
			// A model no entry declares: the name is where the file landed.
			in:         "llamacpp:unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M",
			namespace:  "llamacpp",
			name:       "unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M",
			repository: "unsloth/SmolLM2-135M-Instruct-GGUF",
			fileName:   "SmolLM2-135M-Instruct-Q4_K_M",
		},
		{
			// Nothing forbids a catalog key holding a slash, so it is an id, not a source.
			in: "vendor/slashed-id", namespace: "", name: "vendor/slashed-id",
			repository: "vendor", fileName: "slashed-id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			id, err := Parse(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.namespace, id.Namespace())
			assert.Equal(t, tt.name, id.Name())
			assert.Equal(t, tt.repository, id.Repository())
			assert.Equal(t, tt.fileName, id.FileName())
			assert.Equal(t, tt.in, id.String(), "parsing and printing must round-trip")
		})
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"nothing after the framework", "llamacpp:"},
		{"no framework before the colon", ":gemma-3-1b-it-Q4_0"},
		// A compact source key. Told apart by its second colon, which no id carries.
		{"a source key", "llamacpp:unsloth/repo:Q4_0"},
		{"a traversing segment", "llamacpp:../../etc/passwd"},
		{"an empty segment", "llamacpp:unsloth//file-Q4_0"},
		{"a trailing slash", "llamacpp:unsloth/repo/"},
		{"surrounding whitespace", " llamacpp:gemma-3-1b-it-Q4_0"},
		{"an inner space", "llamacpp:gemma 3 1b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.in)
			require.ErrorIs(t, err, ErrInvalidID)
		})
	}
}

func TestPathSegment(t *testing.T) {
	// The reason GET and DELETE need encoding: a router splits an unescaped name into
	// path segments and matches nothing.
	id, err := Parse("llamacpp:unsloth/repo/file-Q4_0")
	require.NoError(t, err)

	assert.Equal(t, "llamacpp:unsloth%2Frepo%2Ffile-Q4_0", id.PathSegment())
}

func TestMarshalJSON(t *testing.T) {
	id, err := Parse("llamacpp:unsloth/repo/file-Q4_0")
	require.NoError(t, err)

	encoded, err := json.Marshal(struct{ ID ID }{id})
	require.NoError(t, err)
	assert.JSONEq(t, `{"ID":"llamacpp:unsloth/repo/file-Q4_0"}`, string(encoded))
}

func TestEqual(t *testing.T) {
	id, err := Parse("llamacpp:unsloth/repo/file-Q4_0")
	require.NoError(t, err)
	same, err := Parse("llamacpp:unsloth/repo/file-Q4_0")
	require.NoError(t, err)
	other, err := Parse("llamacpp:file-Q4_0")
	require.NoError(t, err)

	assert.True(t, id.Equal(same))
	assert.False(t, id.Equal(other), "the repository is part of the identity")
}

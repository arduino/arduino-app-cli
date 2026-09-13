// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModelIDRoundTrip pins the one spelling an id has on the wire: EncodeModelID is what
// a response reports as "id", and DecodeModelID is the only way back in.
func TestModelIDRoundTrip(t *testing.T) {
	for _, id := range []string{
		"face-detection",
		"llamacpp:Qwen3.5-0.8B-Q4_0",
		"ei-model-901144-1",
		"vendor/slashed-id",
		"llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0",
	} {
		encoded := EncodeModelID(id)
		assert.NotContains(t, encoded, "/", "an encoded id is one path segment: %q", id)

		got, err := DecodeModelID(encoded)
		require.NoError(t, err)
		assert.Equal(t, id, got)
	}

	// A padded encoding is not the form EncodeModelID produces, so it is refused.
	_, err := DecodeModelID(EncodeModelID("face-detection") + "=")
	assert.Error(t, err)

	// An id carrying ":" is not valid base64url, so the plain form is refused. One without
	// it decodes to no model and takes the not-found answer instead.
	_, err = DecodeModelID("llamacpp:Qwen3.5-0.8B-Q4_0")
	assert.Error(t, err)
}

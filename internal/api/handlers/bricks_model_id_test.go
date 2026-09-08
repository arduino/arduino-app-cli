// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
)

// TestDecodeRequestModel covers the other door a model id comes in by: a brick request
// names one in its body, in the same base64url form the path takes.
func TestDecodeRequestModel(t *testing.T) {
	t.Run("an encoded id becomes the plain one", func(t *testing.T) {
		encoded := models.EncodeModelID("llamacpp:owner/repo/file")
		req := bricks.BrickCreateUpdateRequest{Model: &encoded}

		require.NoError(t, decodeRequestModel(&req))
		assert.Equal(t, "llamacpp:owner/repo/file", *req.Model)
	})

	t.Run("a request naming no model is left alone", func(t *testing.T) {
		req := bricks.BrickCreateUpdateRequest{}
		require.NoError(t, decodeRequestModel(&req))
		assert.Nil(t, req.Model)

		empty := ""
		req = bricks.BrickCreateUpdateRequest{Model: &empty}
		require.NoError(t, decodeRequestModel(&req))
		assert.Equal(t, "", *req.Model)
	})

	t.Run("an id that is not base64url is refused", func(t *testing.T) {
		plain := "llamacpp:owner/repo/file"
		req := bricks.BrickCreateUpdateRequest{Model: &plain}

		err := decodeRequestModel(&req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base64url")
	})
}

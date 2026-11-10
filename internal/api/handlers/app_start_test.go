// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSavePrebuiltSketch(t *testing.T) {
	t.Run("file field", func(t *testing.T) {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("file", "sketch-prebuilt.fw")
		require.NoError(t, err)
		_, err = part.Write([]byte("fw content"))
		require.NoError(t, err)
		require.NoError(t, mw.Close())

		r := httptest.NewRequest(http.MethodPost, "/v1/apps/x/start?prebuilt_sketch=true", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())

		fwFile, err := savePrebuiltSketch(r)
		require.NoError(t, err)
		t.Cleanup(func() { _ = fwFile.Remove() })
		content, err := fwFile.ReadFile()
		require.NoError(t, err)
		require.Equal(t, "fw content", string(content))
	})

	t.Run("no file field", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/v1/apps/x/start?prebuilt_sketch=true", bytes.NewBufferString("{}"))
		r.Header.Set("Content-Type", "application/json")

		_, err := savePrebuiltSketch(r)
		require.Error(t, err)
	})
}

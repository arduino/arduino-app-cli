// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/models"
)

type buildResponsePart struct {
	header map[string][]string
	body   []byte
}

// readBuildResponse reads the parts of a recorded build response.
func readBuildResponse(t *testing.T, recorder *httptest.ResponseRecorder) []buildResponsePart {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	mediaType, params, err := mime.ParseMediaType(recorder.Header().Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/mixed", mediaType)

	var parts []buildResponsePart
	reader := multipart.NewReader(recorder.Body, params["boundary"])
	for {
		part, err := reader.NextRawPart()
		if err == io.EOF {
			return parts
		}
		require.NoError(t, err)
		body, err := io.ReadAll(part)
		require.NoError(t, err)
		parts = append(parts, buildResponsePart{header: part.Header, body: body})
	}
}

func TestBuildResponse(t *testing.T) {
	const heartbeat = 10 * time.Millisecond

	t.Run("the archive is the last part, after the heartbeats", func(t *testing.T) {
		archive := paths.New(t.TempDir(), "app.ard")
		require.NoError(t, archive.WriteFile([]byte("archive content")))

		recorder := httptest.NewRecorder()
		response := startBuildResponse(recorder, heartbeat)
		time.Sleep(5 * heartbeat)
		require.NoError(t, response.SendArchive(archive, "app.ard"))
		require.True(t, recorder.Flushed)

		parts := readBuildResponse(t, recorder)
		require.Greater(t, len(parts), 2, "no heartbeat was sent while the build was running")
		for _, part := range parts[:len(parts)-1] {
			assert.Equal(t, []string{"text/plain"}, part.header["Content-Type"])
			assert.Empty(t, part.body)
		}
		last := parts[len(parts)-1]
		assert.Equal(t, []string{"application/gzip"}, last.header["Content-Type"])
		assert.Equal(t, []string{`attachment; filename="app.ard"`}, last.header["Content-Disposition"])
		assert.Equal(t, "archive content", string(last.body))
	})

	t.Run("a failed build ends with the error in place of the archive", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		response := startBuildResponse(recorder, heartbeat)
		response.SendError(models.ErrorResponse{Code: "INTERNAL_SERVER_ERROR", Details: "the build failed"})

		parts := readBuildResponse(t, recorder)
		require.NotEmpty(t, parts)
		last := parts[len(parts)-1]
		assert.Equal(t, []string{"application/json"}, last.header["Content-Type"])
		var got models.ErrorResponse
		require.NoError(t, json.Unmarshal(last.body, &got))
		assert.Equal(t, models.ErrorResponse{Code: "INTERNAL_SERVER_ERROR", Details: "the build failed"}, got)
	})

	t.Run("no heartbeat is sent once the response is over", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		response := startBuildResponse(recorder, heartbeat)
		response.SendError(models.ErrorResponse{Details: "the build failed"})
		sent := recorder.Body.Len()
		time.Sleep(5 * heartbeat)
		assert.Equal(t, sent, recorder.Body.Len())
	})
}

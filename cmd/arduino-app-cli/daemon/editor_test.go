// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestWithEditor(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h, err := withEditor(api, []string{"http://localhost:*"})
	require.NoError(t, err)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	editURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/edit"

	t.Run("the API keeps its routes", func(t *testing.T) {
		res, err := http.Get(srv.URL + "/v1/apps")
		require.NoError(t, err)
		require.NoError(t, res.Body.Close())
		require.Equal(t, http.StatusTeapot, res.StatusCode)
	})

	t.Run("an allowed origin opens a session", func(t *testing.T) {
		c, _, err := websocket.DefaultDialer.Dial(editURL, http.Header{"Origin": {"http://localhost:5173"}})
		require.NoError(t, err)
		defer c.Close()
		require.NoError(t, c.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "hello", "params": map[string]any{}}))
		var reply struct {
			Result struct {
				Protocol string `json:"protocol"`
			} `json:"result"`
		}
		require.NoError(t, c.ReadJSON(&reply))
		require.Equal(t, "editor.v1", reply.Result.Protocol)
	})

	t.Run("another origin is refused", func(t *testing.T) {
		_, res, err := websocket.DefaultDialer.Dial(editURL, http.Header{"Origin": {"https://example.com"}})
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, res.StatusCode)
	})
}

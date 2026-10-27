// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package editor_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/gorilla/websocket"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/editor"
)

type notif struct {
	Method string
	Params json.RawMessage
}

func dial(t *testing.T) (*jsonrpc2.Conn, chan notif, func()) {
	t.Helper()
	h, err := editor.New(editor.Config{MaxWatches: 16, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	require.NoError(t, err)
	srv := httptest.NewServer(h)
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)

	ns := make(chan notif, 16)
	handler := jsonrpc2.HandlerWithError(func(_ context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
		if req.Notif && req.Params != nil {
			ns <- notif{Method: req.Method, Params: *req.Params}
		}
		return nil, nil
	})
	rpc := jsonrpc2.NewConn(context.Background(), wsStream{c}, handler)
	return rpc, ns, func() { _ = rpc.Close(); _ = c.Close(); srv.Close() }
}

// wsStream duplicates the tiny adapter from editor.go; kept here so tests
// don't need to import an unexported type.
type wsStream struct{ c *websocket.Conn }

func (s wsStream) WriteObject(o interface{}) error {
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return s.c.WriteMessage(websocket.TextMessage, b)
}

func (s wsStream) ReadObject(v interface{}) error {
	for {
		mt, d, err := s.c.ReadMessage()
		if err != nil {
			return err
		}
		if mt == websocket.TextMessage || mt == websocket.BinaryMessage {
			return json.Unmarshal(d, v)
		}
	}
}

func (s wsStream) Close() error { return s.c.Close() }

func TestServer_Hello(t *testing.T) {
	rpc, _, cleanup := dial(t)
	defer cleanup()
	var res struct {
		Protocol     string   `json:"protocol"`
		Capabilities []string `json:"capabilities"`
		Limits       struct {
			MaxWatches int `json:"maxWatches"`
		} `json:"limits"`
	}
	require.NoError(t, rpc.Call(context.Background(), "hello", nil, &res))
	assert.Equal(t, "editor.v1", res.Protocol)
	assert.Contains(t, res.Capabilities, "fs.watch")
	for _, m := range []string{"fs.stat", "fs.read", "fs.write", "fs.mkdir", "fs.delete"} {
		assert.Contains(t, res.Capabilities, m)
	}
	assert.Equal(t, 16, res.Limits.MaxWatches)
}

func TestServer_WriteReadStatRoundTrip(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, _, cleanup := dial(t)
	defer cleanup()
	target := filepath.Join(root, "main.go")

	var wres struct {
		ETag string `json:"etag"`
		Size int64  `json:"size"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.write",
		map[string]any{"path": target, "encoding": "utf-8", "content": "package main\n"}, &wres))
	assert.Equal(t, int64(len("package main\n")), wres.Size)
	require.NotEmpty(t, wres.ETag)

	var rres struct {
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
		ETag     string `json:"etag"`
		Size     int64  `json:"size"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.read", map[string]any{"path": target}, &rres))
	assert.Equal(t, "utf-8", rres.Encoding)
	assert.Equal(t, "package main\n", rres.Content)
	assert.Equal(t, wres.ETag, rres.ETag)

	var sres struct {
		Size  int64 `json:"size"`
		IsDir bool  `json:"isDir"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.stat", map[string]any{"path": target}, &sres))
	assert.Equal(t, wres.Size, sres.Size)
	assert.False(t, sres.IsDir)
}

func TestServer_ReadNotModifiedOnMatchingBaseEtag(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, _, cleanup := dial(t)
	defer cleanup()
	target := filepath.Join(root, "f.txt")

	var wres struct {
		ETag string `json:"etag"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.write",
		map[string]any{"path": target, "encoding": "utf-8", "content": "hello"}, &wres))

	var rres struct {
		NotModified bool   `json:"notModified"`
		ETag        string `json:"etag"`
		Content     string `json:"content"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.read",
		map[string]any{"path": target, "baseEtag": wres.ETag}, &rres))
	assert.True(t, rres.NotModified)
	assert.Equal(t, wres.ETag, rres.ETag)
	assert.Empty(t, rres.Content)
}

func TestServer_WriteRejectsAStaleBaseEtag(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, _, cleanup := dial(t)
	defer cleanup()
	target := filepath.Join(root, "f.txt")

	var wres struct {
		ETag string `json:"etag"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.write",
		map[string]any{"path": target, "encoding": "utf-8", "content": "v1"}, &wres))

	err := rpc.Call(context.Background(), "fs.write",
		map[string]any{"path": target, "encoding": "utf-8", "content": "v2", "baseEtag": "stale"}, &wres)
	require.Error(t, err)
	jerr, ok := err.(*jsonrpc2.Error)
	require.True(t, ok)
	assert.Equal(t, int64(-32002), jerr.Code)
}

func TestServer_MkdirThenDelete(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, _, cleanup := dial(t)
	defer cleanup()
	dir := filepath.Join(root, "a", "b")

	var out struct{}
	require.NoError(t, rpc.Call(context.Background(), "fs.mkdir",
		map[string]any{"path": dir, "recursive": true}, &out))
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	require.NoError(t, rpc.Call(context.Background(), "fs.delete",
		map[string]any{"path": filepath.Join(root, "a"), "recursive": true}, &out))
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}

func TestServer_StatMissingPathIsNotFound(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, _, cleanup := dial(t)
	defer cleanup()

	var out struct{}
	err := rpc.Call(context.Background(), "fs.stat",
		map[string]any{"path": filepath.Join(root, "nope")}, &out)
	require.Error(t, err)
	jerr, ok := err.(*jsonrpc2.Error)
	require.True(t, ok)
	assert.Equal(t, int64(-32005), jerr.Code)
}

func TestServer_WatchProducesNotification(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, notifs, cleanup := dial(t)
	defer cleanup()

	var wr struct {
		SubscriptionID string `json:"subscriptionId"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.watch", map[string]any{
		"path": root, "recursive": true, "debounceMs": 30,
	}, &wr))
	require.NotEmpty(t, wr.SubscriptionID)

	time.Sleep(80 * time.Millisecond)
	require.NoError(t, os.WriteFile(filepath.Join(root, "hello.txt"), []byte("x"), 0o600))

	select {
	case n := <-notifs:
		assert.Equal(t, "fs.changed", n.Method)
		var payload struct {
			SubscriptionID string `json:"subscriptionId"`
			Events         []struct {
				Type, Path string
			} `json:"events"`
		}
		require.NoError(t, json.Unmarshal(n.Params, &payload))
		assert.Equal(t, wr.SubscriptionID, payload.SubscriptionID)
		require.NotEmpty(t, payload.Events)
		assert.Equal(t, filepath.Join(root, "hello.txt"), payload.Events[0].Path)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for fs.changed")
	}
}

func TestServer_WatchIsIdempotentPerConfig(t *testing.T) {
	// Two identical calls must resolve to one subscription, which is what lets a
	// client re-issue its watch set after a reconnect without tracking what it
	// already sent.
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, notifs, cleanup := dial(t)
	defer cleanup()

	watch := func(params map[string]any) string {
		var wr struct {
			SubscriptionID string `json:"subscriptionId"`
		}
		require.NoError(t, rpc.Call(context.Background(), "fs.watch", params, &wr))
		require.NotEmpty(t, wr.SubscriptionID)
		return wr.SubscriptionID
	}

	cfg := map[string]any{"path": root, "recursive": true, "debounceMs": 30}
	first := watch(cfg)
	assert.Equal(t, first, watch(cfg), "identical params must reuse the subscription")

	// Glob order carries no meaning, so an equivalent request still hits.
	withGlobs := func(ex ...string) map[string]any {
		return map[string]any{
			"path": root, "recursive": true, "debounceMs": 30, "excludes": ex,
		}
	}
	globbed := watch(withGlobs("**/.git/**", "**/node_modules/**"))
	assert.Equal(t, globbed, watch(withGlobs("**/node_modules/**", "**/.git/**")))
	assert.NotEqual(t, first, globbed, "different excludes are a different watch")

	// Shallow and recursive on one path stay distinct: that is why the index is
	// keyed on config rather than path.
	shallow := watch(map[string]any{"path": root, "recursive": false, "debounceMs": 30})
	assert.NotEqual(t, first, shallow)

	// One write, and the recursive subscription reports it exactly once.
	time.Sleep(80 * time.Millisecond)
	require.NoError(t, os.WriteFile(filepath.Join(root, "hello.txt"), []byte("x"), 0o600))

	seen := 0
	deadline := time.After(1500 * time.Millisecond)
	for {
		select {
		case n := <-notifs:
			var payload struct {
				SubscriptionID string `json:"subscriptionId"`
				Events         []struct {
					Type, Path string
				} `json:"events"`
			}
			require.NoError(t, json.Unmarshal(n.Params, &payload))
			if n.Method == "fs.changed" && payload.SubscriptionID == first {
				seen++
			}
		case <-deadline:
			assert.Equal(t, 1, seen, "the deduped subscription reported the write once")
			return
		}
	}
}

func TestServer_UnwatchReleasesTheConfig(t *testing.T) {
	// The index must not outlive the subscription, or a re-watch would hand back
	// a dead id.
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, _, cleanup := dial(t)
	defer cleanup()

	cfg := map[string]any{"path": root, "recursive": true, "debounceMs": 30}
	var wr struct {
		SubscriptionID string `json:"subscriptionId"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.watch", cfg, &wr))
	first := wr.SubscriptionID

	var out struct{}
	require.NoError(t, rpc.Call(context.Background(), "fs.unwatch",
		map[string]any{"subscriptionId": first}, &out))

	require.NoError(t, rpc.Call(context.Background(), "fs.watch", cfg, &wr))
	assert.NotEqual(t, first, wr.SubscriptionID, "a fresh subscription, not the dead id")
}

func TestServer_RewatchAfterUnwatchStaysLive(t *testing.T) {
	// Unwatch cancels the context, so pump's teardown races the re-watch below.
	// Clearing the index unconditionally there would evict the new
	// subscription's entry; events still arriving prove it did not.
	root := paths.New(t.TempDir()).Canonical().String()
	rpc, notifs, cleanup := dial(t)
	defer cleanup()

	cfg := map[string]any{"path": root, "recursive": true, "debounceMs": 30}
	var wr struct {
		SubscriptionID string `json:"subscriptionId"`
	}
	require.NoError(t, rpc.Call(context.Background(), "fs.watch", cfg, &wr))
	first := wr.SubscriptionID

	var out struct{}
	require.NoError(t, rpc.Call(context.Background(), "fs.unwatch",
		map[string]any{"subscriptionId": first}, &out))
	require.NoError(t, rpc.Call(context.Background(), "fs.watch", cfg, &wr))
	second := wr.SubscriptionID
	require.NotEqual(t, first, second)

	time.Sleep(120 * time.Millisecond)
	require.NoError(t, os.WriteFile(filepath.Join(root, "live.txt"), []byte("x"), 0o600))

	for {
		select {
		case n := <-notifs:
			var payload struct {
				SubscriptionID string `json:"subscriptionId"`
			}
			require.NoError(t, json.Unmarshal(n.Params, &payload))
			if n.Method == "fs.changed" && payload.SubscriptionID == second {
				return // the replacement is delivering
			}
		case <-time.After(3 * time.Second):
			t.Fatal("re-watched subscription never delivered")
		}
	}
}

func TestServer_UnknownSubscriptionErrors(t *testing.T) {
	rpc, _, cleanup := dial(t)
	defer cleanup()
	var out struct{}
	err := rpc.Call(context.Background(), "fs.unwatch", map[string]any{"subscriptionId": "sub-nope"}, &out)
	require.Error(t, err)
	jerr, ok := err.(*jsonrpc2.Error)
	require.True(t, ok)
	assert.Equal(t, int64(-32602), jerr.Code)
}

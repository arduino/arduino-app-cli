// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package editor

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// newTestSub is a helper that spins up a recursive subscription rooted at dir
// with a short debounce, ready to receive events.
func newTestSub(t *testing.T, dir string, opts watchParams) *watchSub {
	t.Helper()
	if opts.DebounceMs == 0 {
		opts.DebounceMs = 30
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	rootCanon := paths.New(dir).Canonical().String()
	sub, err := newWatchSub(ctx, paths.New(rootCanon), opts, testLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.fsw.Close() })
	time.Sleep(50 * time.Millisecond) // give fsnotify a moment
	return sub
}

func waitForPath(t *testing.T, sub *watchSub, wantPath string, wantType string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case ev := <-sub.events:
			for _, e := range ev {
				if e.Path.String() == wantPath && (wantType == "" || e.Type == wantType) {
					return
				}
			}
		case <-time.After(200 * time.Millisecond):
		}
	}
	t.Fatalf("timeout waiting for %q (%s)", wantPath, wantType)
}

func TestWatch_UpdateEvent(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	f := filepath.Join(root, "hello.txt")
	require.NoError(t, os.WriteFile(f, []byte("hi"), 0o600))
	sub := newTestSub(t, root, watchParams{Recursive: true})
	require.NoError(t, os.WriteFile(f, []byte("hi2"), 0o600))
	waitForPath(t, sub, f, "update")
}

// Reproduces vim's default `:w` on Linux (backupcopy=no): rename the original
// out of the way, then write a new file at the same path, then unlink the
// backup. On the target path fsnotify emits Rename+Create+Write in one
// debounce window; without special handling coalesce would drop the target
// entirely. It must surface as "update".
func TestWatch_AtomicReplaceSaveEmitsUpdate(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	f := filepath.Join(root, "asd")
	backup := filepath.Join(root, ".asd~")
	require.NoError(t, os.WriteFile(f, []byte("v1"), 0o600))
	sub := newTestSub(t, root, watchParams{Recursive: true})

	require.NoError(t, os.Rename(f, backup))
	require.NoError(t, os.WriteFile(f, []byte("v2"), 0o600))
	require.NoError(t, os.Remove(backup))

	waitForPath(t, sub, f, "update")
}

func TestWatch_CreateAndDelete(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	sub := newTestSub(t, root, watchParams{Recursive: true})
	f := filepath.Join(root, "new.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
	waitForPath(t, sub, f, "create")
	require.NoError(t, os.Remove(f))
	waitForPath(t, sub, f, "delete")
}

func TestWatch_RecursiveNestedCreateRace(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	sub := newTestSub(t, root, watchParams{Recursive: true})
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a", "b", "c"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a", "b", "c", "file.txt"), []byte("x"), 0o600))
	waitForPath(t, sub, filepath.Join(root, "a", "b", "c", "file.txt"), "")
}

func TestWatch_Excludes(t *testing.T) {
	root := paths.New(t.TempDir()).Canonical().String()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "node_modules"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o755))
	sub := newTestSub(t, root, watchParams{Recursive: true, Excludes: []string{"node_modules/**"}})
	require.NoError(t, os.WriteFile(filepath.Join(root, "node_modules", "a.js"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "a.ts"), []byte("x"), 0o600))

	wantSrc := filepath.Join(root, "src", "a.ts")
	wantNM := filepath.Join(root, "node_modules", "a.js")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case ev := <-sub.events:
			for _, e := range ev {
				if e.Path.String() == wantSrc {
					return
				}
				if e.Path.String() == wantNM {
					t.Fatalf("excluded event leaked: %+v", e)
				}
			}
		case <-time.After(200 * time.Millisecond):
		}
	}
	t.Fatal("did not observe expected src/a.ts event")
}

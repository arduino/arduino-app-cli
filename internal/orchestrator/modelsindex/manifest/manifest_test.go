// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
)

// writeManifest writes a manifest plus the artifacts it declares so
// the manifest verifies on read. truncate optionally shortens one of
// the artifacts to simulate a partial download.
func writeManifest(t *testing.T, dir, name string, files map[string]int64, truncate string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	enc := []byte(`{"version":1,"files":[`)
	first := true
	for rel, size := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		actual := size
		if rel == truncate {
			actual = size - 1
		}
		if err := os.WriteFile(full, make([]byte, actual), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
		if !first {
			enc = append(enc, ',')
		}
		first = false
		enc = append(enc, []byte(`{"path":"`+rel+`","size":`+itoa(size)+`}`)...)
	}
	enc = append(enc, []byte(`]}`)...)
	mpath := filepath.Join(dir, name)
	if err := os.WriteFile(mpath, enc, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return mpath
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestFindReturnsVerifiedManifestsRecursively(t *testing.T) {
	root := t.TempDir()

	// AI Hub-style: dedicated subdir + fixed-name manifest.
	aiDir := filepath.Join(root, "audio-analytics/tts/melotts_es")
	writeManifest(t, aiDir, "downloaded.json", map[string]int64{
		"model.so": 1024,
		"config":   16,
	}, "")

	// EI-style: two sidecar manifests co-located in another dir.
	eiDir := filepath.Join(root, "edge-impulse")
	writeManifest(t, eiDir, "efficientnet-b4-qnn.eim.downloaded.json", map[string]int64{
		"efficientnet-b4-qnn.eim": 2048,
	}, "")
	writeManifest(t, eiDir, "yolo-x-small-qnn.eim.downloaded.json", map[string]int64{
		"yolo-x-small-qnn.eim": 2048,
	}, "")

	got := Find(paths.New(root))
	if len(got) != 3 {
		t.Fatalf("Find returned %d manifests, want 3", len(got))
	}
	byPath := map[string]Manifest{}
	for _, m := range got {
		byPath[m.Filename()+"@"+filepath.Base(m.Dir)] = m
	}
	if m, ok := byPath["downloaded.json@melotts_es"]; !ok || m.TotalSize() != 1024+16 {
		t.Fatalf("AI Hub manifest missing or wrong size: %+v", m)
	}
	if _, ok := byPath["efficientnet-b4-qnn.eim.downloaded.json@edge-impulse"]; !ok {
		t.Fatalf("efficientnet sidecar missing")
	}
	if _, ok := byPath["yolo-x-small-qnn.eim.downloaded.json@edge-impulse"]; !ok {
		t.Fatalf("yolo sidecar missing")
	}
}

func TestFindSkipsManifestsThatFailVerification(t *testing.T) {
	root := t.TempDir()

	writeManifest(t, filepath.Join(root, "ok"), "downloaded.json", map[string]int64{"a": 8}, "")
	writeManifest(t, filepath.Join(root, "truncated"), "downloaded.json", map[string]int64{"a": 8}, "a")

	missingDir := filepath.Join(root, "missing")
	if err := os.MkdirAll(missingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(missingDir, "downloaded.json"),
		[]byte(`{"version":1,"files":[{"path":"ghost","size":1}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := Find(paths.New(root))
	if len(got) != 1 {
		t.Fatalf("Find returned %d manifests, want 1 (only the valid one)", len(got))
	}
	if filepath.Base(got[0].Dir) != "ok" {
		t.Fatalf("expected manifest from 'ok' dir, got %q", got[0].Dir)
	}
}

func TestFindEmptyRoots(t *testing.T) {
	if got := Find(nil); len(got) != 0 {
		t.Fatalf("Find(nil) returned %d manifests", len(got))
	}
	if got := Find(paths.New(filepath.Join(t.TempDir(), "does-not-exist"))); len(got) != 0 {
		t.Fatalf("Find(missing) returned %d manifests", len(got))
	}
}

func TestManifestAbsPathsIncludesManifestItself(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub")
	mpath := writeManifest(t, dir, "downloaded.json", map[string]int64{"x": 4}, "")
	m, err := Read(mpath)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	abs := m.AbsPaths()
	if len(abs) != 2 {
		t.Fatalf("AbsPaths returned %d entries, want 2", len(abs))
	}
	if filepath.Base(abs[len(abs)-1]) != "downloaded.json" {
		t.Fatalf("last AbsPath should be the manifest itself: %v", abs)
	}
}

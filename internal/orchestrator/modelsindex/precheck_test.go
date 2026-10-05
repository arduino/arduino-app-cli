// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"math"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestParseInfoLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		size    uint64
		errText string
		isErr   bool
	}{
		{"HF stat: bytes win over MiB", `{"event":"stat","size_bytes":1000,"size_mb":5}`, 1000, "", false},
		{"zero bytes falls back to MiB", `{"event":"stat","size_bytes":0,"size_mb":46}`, mibToBytes(46), "", false},
		{"AI Hub stat: MiB only", `{"event":"stat","size_mb":46}`, mibToBytes(46), "", false},
		{"null size is unknown", `{"event":"stat","size_mb":null}`, 0, "", false},
		{"-1 is unknown, not a parse error", `{"event":"stat","size_bytes":-1}`, 0, "", false},
		{"error with text", `{"event":"error","description":"gated"}`, 0, "gated", true},
		{"error without text is still an error", `{"event":"error"}`, 0, "", true},
		{"other events are neither", `{"event":"info","description":"x"}`, 0, "", false},
		{"not JSON", `Downloading...`, 0, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			size, text, isErr := parseInfoLine(tc.line)
			assert.Equal(t, tc.size, size)
			assert.Equal(t, tc.errText, text)
			assert.Equal(t, tc.isErr, isErr)
		})
	}
}

// TestClassifyHandlerError uses the sentences hf_downloader.py prints today, whole, so a
// rewording on the container side shows up here as a row falling to ErrInfoFailed.
func TestClassifyHandlerError(t *testing.T) {
	tests := []struct {
		text string
		want error
	}{
		{"Invalid Hugging Face URL: foo\nnot a link\nuse https://huggingface.co/...", ErrBadModelURL},
		{"Hugging Face model repository 'a/b' does not exist, or is not public. Only public model repositories can be downloaded.", ErrModelNotFound},
		{"Hugging Face repository 'a/b' is gated: its files are only served after its conditions are accepted.", ErrModelForbidden},
		{"Hugging Face repository 'a/b' is private. Only public model repositories can be downloaded.", ErrModelForbidden},
		{"Hugging Face repository 'a/b' has been disabled by its authors.", ErrModelGone},
		{"Revision 'x' does not exist in Hugging Face repository 'a/b'.", ErrModelNotFound},
		{"File 'm.gguf' does not exist in Hugging Face repository 'a/b', which contains no GGUF files at all.", ErrNoCompatibleFile},
		{"No file matching '*Q4_0*.gguf' found in repository 'a/b'.", ErrModelNotFound},
		{"Cannot download model. Not supported quantization: Q2_K.", ErrUnsupportedModel},
		{"Unsupported model architecture 'bert' in Hugging Face repository 'a/b'.", ErrUnsupportedModel},
		{"Could not verify Hugging Face repository 'a/b': connection refused", ErrHubUnreachable},
		{"something nobody has seen before", ErrInfoFailed},
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			err := ClassifyHandlerError(tc.text)
			require.ErrorIs(t, err, tc.want)
			assert.Contains(t, err.Error(), tc.text, "the container's words reach the user")
		})
	}
}

// TestPrecheck drives precheck against a fake container: info answers first, then check.
func TestPrecheck(t *testing.T) {
	const url = "llamacpp:org/repo:Q4_0"
	plat := platform.Platform{BoardName: "ventunoq"}
	// listing is what the listing container reports: none by default.
	newIndex := func(t *testing.T, info, check string, checkExit int, listing ...string) (*ModelsIndex, *fakeDockerClient) {
		t.Helper()
		cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
			switch {
			case len(cmd) > 0 && cmd[0] == listModelsCmd:
				return listingWith(listing...), 0
			case len(cmd) > 0 && strings.Contains(cmd[0], "_info.sh"):
				return info + "\n", 0
			case len(cmd) > 0 && strings.Contains(cmd[0], "_checker.sh"):
				return check + "\n", checkExit
			}
			t.Errorf("unexpected container %v", cmd)
			return "", 1
		})
		dir := paths.New("testdata/with-handlers")
		idx, err := Load(plat, dir, paths.New(t.TempDir()), dir.Join("custom-models"), cli, config.Configuration{})
		require.NoError(t, err)
		idx.locksDir = paths.New(t.TempDir())
		return idx, cli
	}
	const notInstalled = `{"event":"error","description":"Model does not exist","downloading":false}`

	t.Run("a model nobody has: proceed, with the size info reported", func(t *testing.T) {
		idx, cli := newIndex(t, `{"event":"stat","size_bytes":1024}`, notInstalled, 1)
		unlock, res, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.NoError(t, err)
		defer unlock()
		assert.Equal(t, PrecheckResult{SizeBytes: 1024}, res)
	})

	t.Run("bigger than the board's RAM allows: too large, before the disk check", func(t *testing.T) {
		orig := totalMemory
		t.Cleanup(func() { totalMemory = orig })
		totalMemory = func() (uint64, error) { return 1000, nil }

		idx, cli := newIndex(t, `{"event":"stat","size_bytes":751}`, notInstalled, 1)
		_, _, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.ErrorIs(t, err, ErrModelTooLarge)
		assert.Contains(t, err.Error(), "ventunoq")

		idx, cli = newIndex(t, `{"event":"stat","size_bytes":750}`, notInstalled, 1)
		unlock, _, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.NoError(t, err, "at the limit still fits")
		unlock()
	})

	t.Run("info fails: classified, and the lock released", func(t *testing.T) {
		idx, cli := newIndex(t, `{"event":"error","description":"Hugging Face repository 'org/repo' is gated: no"}`, notInstalled, 1)
		_, _, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.ErrorIs(t, err, ErrModelForbidden)
		unlock, err := lockModel(idx.locksDir, url)
		require.NoError(t, err, "a refused precheck must not keep the lock")
		unlock()
	})

	t.Run("info fails for a curated model: always ErrInfoFailed", func(t *testing.T) {
		idx, cli := newIndex(t, `{"event":"error","description":"Hugging Face repository 'org/repo' is gated: no"}`, notInstalled, 1)
		_, _, err := idx.precheck(t.Context(), cli, userHFModel(url, "", plat), "curated-key", false, plat)
		require.ErrorIs(t, err, ErrInfoFailed)
		assert.NotErrorIs(t, err, ErrModelForbidden)
	})

	const exists = `{"event":"info","description":"Model exists","downloading":false}`

	t.Run("on disk and recorded from this link: Installed", func(t *testing.T) {
		// downloadedEntry's record names this very link as its model_url.
		idx, cli := newIndex(t, `{"event":"stat","size_bytes":1024}`, exists, 0, downloadedEntry)
		unlock, res, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.NoError(t, err)
		defer unlock()
		assert.True(t, res.Installed)
	})

	t.Run("on disk but no record of it: a leftover, download again", func(t *testing.T) {
		// Seen on a board: a download stopped after its file landed, before its record.
		// check finds the file, the listing does not list it, so a 409 "already
		// installed" would refuse a model nobody can see or delete.
		idx, cli := newIndex(t, `{"event":"stat","size_bytes":1024}`, exists, 0)
		unlock, res, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.NoError(t, err)
		defer unlock()
		assert.False(t, res.Installed)
	})

	t.Run("a curated install is not confirmed against the listing", func(t *testing.T) {
		// Curated models are listed from their declaration, record or not: check is enough.
		idx, cli := newIndex(t, `{"event":"stat","size_bytes":1024}`, exists, 0)
		unlock, res, err := idx.precheck(t.Context(), cli, userHFModel(url, "", plat), "curated-key", false, plat)
		require.NoError(t, err)
		defer unlock()
		assert.True(t, res.Installed)
	})

	t.Run("a download marker under our lock is a leftover: proceed", func(t *testing.T) {
		idx, cli := newIndex(t, `{"event":"stat","size_bytes":1024}`, `{"event":"info","description":"Model downloading","downloading":true}`, 0)
		unlock, res, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.NoError(t, err)
		defer unlock()
		assert.False(t, res.Installed)
	})

	t.Run("bigger than the free space: ErrInsufficientStorage, lock released", func(t *testing.T) {
		orig := totalMemory
		t.Cleanup(func() { totalMemory = orig })
		totalMemory = func() (uint64, error) { return math.MaxUint64, nil }

		idx, cli := newIndex(t, `{"event":"stat","size_bytes":1152921504606846976}`, notInstalled, 1) // 1 EiB
		_, _, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.ErrorIs(t, err, ErrInsufficientStorage)
		unlock, err := lockModel(idx.locksDir, url)
		require.NoError(t, err)
		unlock()
	})

	t.Run("a handler without info: size unknown, no error", func(t *testing.T) {
		idx, cli := newIndex(t, "", notInstalled, 1)
		hf, ok := idx.Handlers.GetHandlerByID(hfHandlerID)
		require.True(t, ok)
		hf.ID, hf.Actions.Info = "no-info-handler", nil
		idx.Handlers.handlers[hf.ID] = hf
		model := userHFModel(url, "", plat)
		model.Deployment.Handler = hf.ID

		unlock, res, err := idx.precheck(t.Context(), cli, model, url, true, plat)
		require.NoError(t, err)
		defer unlock()
		assert.Zero(t, res.SizeBytes)
	})
}

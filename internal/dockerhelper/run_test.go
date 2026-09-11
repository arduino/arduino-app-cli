// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRun runs a container to completion the way a model handler and the provisioning
// are run: the output is what the caller reads, and the exit code is the error.
func TestRun(t *testing.T) {
	docker := getDockerCli(t).Client()

	t.Run("the output of the container reaches the writers", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := Run(t.Context(), docker, RunOptions{
			Image:  "busybox:latest",
			Cmd:    []string{"sh", "-c", "echo out; echo err >&2"},
			Stdout: &stdout,
			Stderr: &stderr,
		})
		require.NoError(t, err)
		require.Equal(t, "out\n", stdout.String())
		require.Equal(t, "err\n", stderr.String())
	})

	t.Run("the entrypoint is what the caller states", func(t *testing.T) {
		var stdout bytes.Buffer
		err := Run(t.Context(), docker, RunOptions{
			Image:      "busybox:latest",
			Entrypoint: []string{"/bin/echo", "from the entrypoint"},
			Stdout:     &stdout,
		})
		require.NoError(t, err)
		require.Equal(t, "from the entrypoint\n", stdout.String())
	})

	t.Run("a container that fails is an error stating its code", func(t *testing.T) {
		err := Run(t.Context(), docker, RunOptions{
			Image: "busybox:latest",
			Cmd:   []string{"sh", "-c", "exit 3"},
		})
		require.Error(t, err)
		require.True(t, IsExitError(err), err)
		require.ErrorContains(t, err, "3")
	})

	t.Run("the environment reaches the container, with a writable home", func(t *testing.T) {
		var stdout bytes.Buffer
		err := Run(t.Context(), docker, RunOptions{
			Image:  "busybox:latest",
			Cmd:    []string{"sh", "-c", "echo $A_VARIABLE $HOME"},
			Env:    map[string]string{"A_VARIABLE": "a-value"},
			Stdout: &stdout,
		})
		require.NoError(t, err)
		require.Equal(t, "a-value /tmp\n", stdout.String())
	})
}

// TestPullImages downloads what the board has not, and says how far it got.
func TestPullImages(t *testing.T) {
	docker := getDockerCli(t).Client()
	const image = "busybox:latest"

	// Start from a board that does not have it.
	_, _ = RemoveImage(t.Context(), docker, image)

	var lines []string
	var last int64
	var label string
	var total int64
	require.NoError(t, PullImages(t.Context(), docker, []string{image},
		func(line string) { lines = append(lines, line) },
		func(l string, curr, tot int64) {
			require.GreaterOrEqual(t, curr, last, "the download only moves forward")
			last, label, total = curr, l, tot
		},
	))

	require.Contains(t, lines[0], image)
	require.Contains(t, label, "1/1")
	require.Equal(t, total, last, "the last report is the whole download")

	// The image is there now, so there is nothing to say and nothing to fetch.
	lines = nil
	require.NoError(t, PullImages(t.Context(), docker, []string{image},
		func(line string) { lines = append(lines, line) }, nil))
	require.Empty(t, lines)
}

// TestEngineVersion asks the engine what it is: the board must speak api 1.44 or later.
func TestEngineVersion(t *testing.T) {
	version, apiVersion, err := EngineVersion(t.Context(), getDockerCli(t))
	require.NoError(t, err)
	require.NotEmpty(t, version)
	require.NotEmpty(t, apiVersion)
}

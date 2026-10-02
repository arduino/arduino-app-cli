// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"cmp"
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

// TestModelInstall installs a real model through the CLI, no daemon involved. The
// handler image the download runs is arm64 only.
func TestModelInstall(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skipf("Skipping test: requires arm64 architecture, currently running on %s", runtime.GOARCH)
	}

	modelID := cmp.Or(os.Getenv("E2E_MODEL_ID"), "melo-tts-es")

	modelsDir := e2e.MkTempDir(t, "models")
	cli := e2e.NewArduinoAppCLI(t, e2e.WithModelsDir(modelsDir), e2e.WithBoardName("ventunoq"))
	t.Cleanup(cli.CleanUp)

	t.Run("model is not installed before install", func(t *testing.T) {
		model := findModel(t, cli, modelID)
		require.Equal(t, modelsindex.NotInstalledStatus, model.Status)
	})

	t.Run("install downloads and reports the installed model", func(t *testing.T) {
		stdout, stderr, err := cli.Run(t.Context(), "model", "install", modelID, "--format", "json")
		require.NoError(t, err, "stdout: %s\nstderr: %s", stdout, stderr)

		var result struct {
			Model modelsindex.AIModel `json:"model"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &result))
		require.Equal(t, modelID, result.Model.ID)
		require.Equal(t, modelsindex.InstalledStatus, result.Model.Status)
	})

	t.Run("model is installed after install", func(t *testing.T) {
		model := findModel(t, cli, modelID)
		require.Equal(t, modelsindex.InstalledStatus, model.Status)
	})

	t.Run("installing an unknown model fails", func(t *testing.T) {
		_, stderr, err := cli.Run(t.Context(), "model", "install", "not-a-real-model")
		require.Error(t, err)
		require.Contains(t, stderr, "not in the internal model list")
	})
}

// findModel runs "model list" and returns the entry named id, failing the test if it is
// not there.
func findModel(t *testing.T, cli *e2e.ArduinoAppCLI, id string) modelsindex.AIModel {
	t.Helper()

	stdout, stderr, err := cli.Run(t.Context(), "model", "list", "--format", "json")
	require.NoError(t, err, "stdout: %s\nstderr: %s", stdout, stderr)

	var result struct {
		Models []modelsindex.AIModel `json:"models"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	for _, m := range result.Models {
		if m.ID == id {
			return m
		}
	}
	require.Failf(t, "model not found", "no model with id %q in the list", id)
	return modelsindex.AIModel{}
}

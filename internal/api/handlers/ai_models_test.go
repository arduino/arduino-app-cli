// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

// TestInstalledModel covers the install route's last step: describing what the handler
// just wrote, from the download event and the declaration alone. No listing runs, so
// whatever cannot be answered here cannot be answered at all.
func TestInstalledModel(t *testing.T) {
	const adHocID = "llamacpp:unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M"

	t.Run("a source the model list does not declare becomes a user-configured model", func(t *testing.T) {
		idx := &modelsindex.ModelsIndex{}

		model, ok := installedModel(idx, nil, &modelsindex.DownloadedModel{ID: adHocID, Size: 1024})

		require.True(t, ok)
		assert.Equal(t, adHocID, model.ID)
		assert.Equal(t, "unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M", model.Name)
		assert.Equal(t, modelsindex.InstalledStatus, model.Status)
		assert.Equal(t, uint64(1024), model.Size)
		assert.False(t, model.IsBuiltIn, "a model the user installed must stay deletable")
	})

	t.Run("a file landing where the model list declares it is that declared model", func(t *testing.T) {
		// The path named a Hugging Face source, so nothing was declared up front, but the
		// handler resolved the id against the catalog and named a declared model. Its
		// entry describes it, rather than the bare id the event carried.
		idx := &modelsindex.ModelsIndex{InternalModels: []modelsindex.AIModel{
			{ID: "llamacpp:gemma-3-1b-it-Q4_0", Name: "Gemma 3 1B", Description: "An efficient AI model."},
		}}

		model, ok := installedModel(idx, nil, &modelsindex.DownloadedModel{ID: "llamacpp:gemma-3-1b-it-Q4_0", Size: 2048})

		require.True(t, ok)
		assert.Equal(t, "Gemma 3 1B", model.Name)
		assert.Equal(t, "An efficient AI model.", model.Description)
		assert.Equal(t, modelsindex.InstalledStatus, model.Status)
		assert.Equal(t, uint64(2048), model.Size)
	})

	t.Run("a download naming nothing cannot be described", func(t *testing.T) {
		// A models-downloader too old to report model_id: the model installed, but naming
		// it here would promise an id no later request resolves.
		_, ok := installedModel(&modelsindex.ModelsIndex{}, nil, nil)

		assert.False(t, ok)
	})

	t.Run("a declared model takes the size the event reports", func(t *testing.T) {
		declared := &modelsindex.AIModel{ID: "llamacpp:gemma-3-1b-it-Q4_0", Name: "Gemma 3 1B", Size: 1000}

		model, ok := installedModel(&modelsindex.ModelsIndex{}, declared, &modelsindex.DownloadedModel{ID: declared.ID, Size: 2000})

		require.True(t, ok)
		assert.Equal(t, modelsindex.InstalledStatus, model.Status)
		assert.Equal(t, uint64(2000), model.Size, "the on-disk size is closer than the declared one")
	})

	t.Run("a declared model keeps its declared size when the event carries none", func(t *testing.T) {
		// size_mb is omitted when a file cannot be stat'd. The declaration still holds a
		// model_size_mb, and reporting zero would read as an empty install.
		declared := &modelsindex.AIModel{ID: "llamacpp:gemma-3-1b-it-Q4_0", Name: "Gemma 3 1B", Size: 1000}

		model, ok := installedModel(&modelsindex.ModelsIndex{}, declared, &modelsindex.DownloadedModel{ID: declared.ID})

		require.True(t, ok)
		assert.Equal(t, modelsindex.InstalledStatus, model.Status)
		assert.Equal(t, uint64(1000), model.Size)
	})
}

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

func modelWithVars(id, repo, dir string) AIModel {
	return AIModel{
		ID: id,
		Deployment: &ModelDeployment{
			Handler: "hf-handler",
			Variables: []map[string]PlatformDeploymentConfig{
				{"unoq": {Variables: map[string]string{
					"models_repository": repo,
					"model_directory":   dir,
				}}},
			},
		},
	}
}

func TestAcquireDownload(t *testing.T) {
	plat := platform.Platform{BoardName: "unoq"}

	t.Run("a second download of the same model is refused", func(t *testing.T) {
		idx := &ModelsIndex{}
		model := modelWithVars("llamacpp:Qwen3.5-0.8B-Q4_0", "llamacpp", "unsloth/Qwen3.5-0.8B-GGUF")

		release, ok := idx.AcquireDownload(model, plat)
		require.True(t, ok)

		_, ok = idx.AcquireDownload(model, plat)
		assert.False(t, ok, "the second download should be refused")

		release()
		_, ok = idx.AcquireDownload(model, plat)
		assert.True(t, ok, "releasing should allow a retry")
	})

	// Every quantization of one repo is written into the same directory, so
	// two different models can still collide.
	t.Run("models sharing a repository directory collide", func(t *testing.T) {
		idx := &ModelsIndex{}
		q4 := modelWithVars("llamacpp:Qwen3.5-0.8B-Q4_0", "llamacpp", "unsloth/Qwen3.5-0.8B-GGUF")
		q8 := modelWithVars("llamacpp:Qwen3.5-0.8B-Q8_0", "llamacpp", "unsloth/Qwen3.5-0.8B-GGUF")

		_, ok := idx.AcquireDownload(q4, plat)
		require.True(t, ok)
		_, ok = idx.AcquireDownload(q8, plat)
		assert.False(t, ok, "same directory, so it must be refused")
	})

	t.Run("models in different directories are independent", func(t *testing.T) {
		idx := &ModelsIndex{}
		a := modelWithVars("a", "llamacpp", "unsloth/Qwen3.5-0.8B-GGUF")
		b := modelWithVars("b", "llamacpp", "google/gemma-3-1b-it-GGUF")

		_, ok := idx.AcquireDownload(a, plat)
		require.True(t, ok)
		_, ok = idx.AcquireDownload(b, plat)
		assert.True(t, ok)
	})

	t.Run("falls back to the model ID without deployment variables", func(t *testing.T) {
		idx := &ModelsIndex{}
		model := AIModel{ID: "custom-model"}

		_, ok := idx.AcquireDownload(model, plat)
		require.True(t, ok)
		_, ok = idx.AcquireDownload(model, plat)
		assert.False(t, ok)
	})

	t.Run("exactly one of many concurrent callers wins", func(t *testing.T) {
		idx := &ModelsIndex{}
		model := modelWithVars("m", "llamacpp", "unsloth/Qwen3.5-0.8B-GGUF")

		var wg sync.WaitGroup
		var mu sync.Mutex
		won := 0
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, ok := idx.AcquireDownload(model, plat); ok {
					mu.Lock()
					won++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 1, won)
	})
}

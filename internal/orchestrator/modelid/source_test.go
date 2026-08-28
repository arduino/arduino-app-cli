// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The accepted forms, taken from the handler's own docstring so a divergence shows up
// here rather than inside a container.
func TestParseSource(t *testing.T) {
	tests := []struct {
		name         string
		spec         string
		repoID       string
		quantization string
		defaulted    bool
		mmproj       string
	}{
		{
			name: "a repository alone takes the default quantization",
			spec: "unsloth/Qwen3-0.6B-GGUF", repoID: "unsloth/Qwen3-0.6B-GGUF",
			quantization: "Q4_0", defaulted: true,
		},
		{
			name: "repository and quantization, the llama.cpp -hf shape",
			spec: "Qwen/Qwen3-8B-GGUF:Q8_0", repoID: "Qwen/Qwen3-8B-GGUF", quantization: "Q8_0",
		},
		{
			name: "framework, repository and quantization",
			spec: "llamacpp:Qwen/Qwen3-8B-GGUF:Q8_0", repoID: "Qwen/Qwen3-8B-GGUF", quantization: "Q8_0",
		},
		{
			name:   "a fourth field is the mmproj quantization",
			spec:   "llamacpp:unsloth/gemma-4-E4B-it-GGUF:Q4_0:BF16",
			repoID: "unsloth/gemma-4-E4B-it-GGUF", quantization: "Q4_0", mmproj: "BF16",
		},
		{
			name: "a canonical repository has no owner",
			spec: "gpt2:Q4_0", repoID: "gpt2", quantization: "Q4_0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := ParseSource(tt.spec, "")
			require.NoError(t, err)
			assert.Equal(t, tt.repoID, src.RepoID())
			assert.Equal(t, tt.quantization, src.Quantization())
			assert.Equal(t, tt.defaulted, src.QuantizationDefaulted())
			assert.Equal(t, tt.mmproj, src.MmprojQuantization())
			assert.False(t, src.IsURL())
			assert.Equal(t, map[string]string{"model_url": tt.spec}, src.Variables())
		})
	}
}

func TestParseSourceURL(t *testing.T) {
	const file = "https://huggingface.co/unsloth/SmolLM2-135M-Instruct-GGUF/resolve/main/SmolLM2-135M-Instruct-Q4_K_M.gguf"
	const mmproj = "https://huggingface.co/unsloth/SmolLM2-135M-Instruct-GGUF/blob/main/mmproj-F16.gguf"

	src, err := ParseSource(file, mmproj)
	require.NoError(t, err)

	assert.True(t, src.IsURL())
	assert.Empty(t, src.RepoID(), "the handler derives the repository from the path")
	assert.Equal(t, map[string]string{"model_url": file, "model_mmproj_url": mmproj}, src.Variables())
}

func TestParseSourceRejects(t *testing.T) {
	tests := []struct {
		name      string
		spec      string
		mmprojURL string
	}{
		{name: "empty", spec: ""},
		{name: "five fields", spec: "llamacpp:unsloth/repo:Q4_0:BF16:extra"},
		{name: "an empty repository", spec: ":Q4_0"},
		{name: "an empty quantization", spec: "unsloth/repo:"},
		{name: "a traversing repository", spec: "../../etc:Q4_0"},
		{name: "a repository with too many parts", spec: "a/b/c:Q4_0"},
		{name: "a repository no Hub name allows", spec: "unsloth/repo$:Q4_0"},
		{
			// An installed model's id sent back to the install route. Two fields, so the
			// repository reads as "llamacpp" and the quantization as a path.
			name: "a model id rather than a source",
			spec: "llamacpp:unsloth/repo/file-Q4_0",
		},
		{
			name: "an mmproj url alongside a key",
			spec: "unsloth/repo:Q4_0", mmprojURL: "https://huggingface.co/unsloth/repo/blob/main/mmproj-F16.gguf",
		},
		{name: "a url that is not a gguf", spec: "https://huggingface.co/unsloth/repo/resolve/main/README.md"},
		{name: "a lookalike host", spec: "https://huggingface.co.example.com/unsloth/repo/resolve/main/m.gguf"},
		{name: "credentials hiding the host", spec: "https://huggingface.co@example.com/unsloth/repo/resolve/main/m.gguf"},
		{name: "an explicit port", spec: "https://huggingface.co:8443/unsloth/repo/resolve/main/m.gguf"},
		{name: "a scheme that is not http", spec: "ftp://huggingface.co/unsloth/repo/resolve/main/m.gguf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSource(tt.spec, tt.mmprojURL)
			require.ErrorIs(t, err, ErrInvalidSource)
		})
	}
}

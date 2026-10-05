// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyHandlerError uses the sentences hf_downloader.py prints today, whole, so a
// rewording on the container side shows up here as a row no longer matching.
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
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			err := ClassifyHandlerError(tc.text)
			require.ErrorIs(t, err, tc.want)
			assert.Contains(t, err.Error(), tc.text, "the container's words reach the user")
		})
	}

	t.Run("unknown text is a plain error", func(t *testing.T) {
		assert.EqualError(t, ClassifyHandlerError("something new"), "something new")
	})
}

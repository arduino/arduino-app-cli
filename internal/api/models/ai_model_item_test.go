// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

// TestNewAIModelItem pins every field the API reports to the model it comes from.
func TestNewAIModelItem(t *testing.T) {
	item := NewAIModelItem(modelsindex.AIModel{
		ID:           "llamacpp:org/repo/m-Q4_0",
		Name:         "m",
		Handler:      "hf-handler",
		Description:  "d",
		Runner:       "brick",
		Bricks:       []modelsindex.BrickConfig{{ID: "arduino:llm"}},
		Metadata:     map[string]string{"k": "v"},
		Preinstalled: false,
		Origin:       modelsindex.UserOrigin,
		SizeBytes:    1024,
		Status:       modelsindex.InstalledStatus,
	})
	size := uint64(1024)
	assert.Equal(t, AIModelItem{
		ID:           EncodeModelID("llamacpp:org/repo/m-Q4_0"),
		IDDecoded:    "llamacpp:org/repo/m-Q4_0",
		Name:         "m",
		Handler:      "hf-handler",
		Description:  "d",
		Runner:       "brick",
		Bricks:       []string{"arduino:llm"},
		Metadata:     map[string]string{"k": "v"},
		Preinstalled: false,
		Origin:       modelsindex.UserOrigin,
		SizeBytes:    &size,
		Status:       modelsindex.InstalledStatus,
	}, item)

	assert.Nil(t, NewAIModelItem(modelsindex.AIModel{}).SizeBytes, "an unknown size is left out, not 0")
}

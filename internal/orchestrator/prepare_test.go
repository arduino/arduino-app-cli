// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestPrepareModelsSkipsWhatItShould(t *testing.T) {
	bricksIndex := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{
			{
				ID:           "arduino:ai-brick",
				Name:         "AI brick with a default model",
				RequireModel: true,
				ModelName:    "installed-default-model",
			},
			{
				ID:           "arduino:ai-brick-override",
				Name:         "AI brick whose model the app overrides",
				RequireModel: true,
				ModelName:    "installed-default-model",
			},
			{
				ID:           "arduino:ai-brick-builtin",
				Name:         "AI brick wired to a model the handler does not download",
				RequireModel: true,
				ModelName:    "builtin-model",
			},
			{
				ID:           "arduino:ai-brick-unknown",
				Name:         "AI brick wired to a model the index does not know",
				RequireModel: true,
				ModelName:    "unknown-model",
			},
			{
				ID:   "arduino:plain-brick",
				Name: "Brick that requires no model",
			},
		},
	}

	modelIndex := &modelsindex.ModelsIndex{
		InternalModels: []modelsindex.AIModel{
			{
				ID:         "installed-default-model",
				Status:     modelsindex.InstalledStatus,
				Deployment: &modelsindex.ModelDeployment{Handler: "a-handler"},
				Bricks:     []modelsindex.BrickConfig{{ID: "arduino:ai-brick"}},
			},
			{
				ID:         "installed-override-model",
				Status:     modelsindex.InstalledStatus,
				Deployment: &modelsindex.ModelDeployment{Handler: "a-handler"},
				Bricks:     []modelsindex.BrickConfig{{ID: "arduino:ai-brick-override"}},
			},
			{
				// A pre-loaded model is there by declaration: NeedsNoDownload short-circuits
				// its install even though it reads as not installed.
				ID:         "builtin-model",
				Status:     modelsindex.NotInstalledStatus,
				Deployment: &modelsindex.ModelDeployment{PreLoaded: true},
				Bricks:     []modelsindex.BrickConfig{{ID: "arduino:ai-brick-builtin"}},
			},
		},
	}

	appBricks := []app.Brick{
		{ID: "arduino:ai-brick"},
		{ID: "arduino:ai-brick-override", Model: "installed-override-model"},
		{ID: "arduino:ai-brick-builtin"},
		{ID: "arduino:ai-brick-unknown"},
		{ID: "arduino:plain-brick"},
	}

	var messages []string
	cb := func(m StreamMessage) {
		if m.GetType() == InfoType {
			messages = append(messages, m.GetData())
		}
	}

	err := prepareModels(t.Context(), nil, appBricks, bricksIndex, modelIndex, platform.Platform{}, cb)
	require.NoError(t, err)

	// The installed and the unresolved models are downloaded nothing about: only the
	// pre-loaded one reaches the install, which then finds it needs no download.
	assert.NotContains(t, messages, "downloading the model installed-default-model")
	assert.NotContains(t, messages, "downloading the model installed-override-model")
	assert.NotContains(t, messages, "downloading the model unknown-model")
}

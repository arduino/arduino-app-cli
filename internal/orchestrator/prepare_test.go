// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/containerd/errdefs"
	"github.com/docker/cli/cli/command"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type preparedImageClient struct {
	client.APIClient
	inspect func(string) error
}

func (c preparedImageClient) ImageInspect(_ context.Context, image string, _ ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	return client.ImageInspectResult{}, c.inspect(image)
}

type preparedImageCLI struct {
	command.Cli
	docker client.APIClient
}

func (c preparedImageCLI) Client() client.APIClient { return c.docker }

func TestCheckImages(t *testing.T) {
	daemonError := errors.New("cannot reach Docker")
	for _, tc := range []struct {
		name       string
		inspectErr error
		wantErr    error
	}{
		{name: "local image with implicit latest"},
		{name: "missing image", inspectErr: errdefs.ErrNotFound, wantErr: ErrNotPrepared},
		{name: "daemon failure", inspectErr: daemonError, wantErr: daemonError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			docker := preparedImageCLI{docker: preparedImageClient{inspect: func(image string) error {
				require.Equal(t, "viktoruj/ping_pong", image)
				return tc.inspectErr
			}}}
			prj := &types.Project{Services: types.Services{
				"main": {Image: "viktoruj/ping_pong"},
			}}
			err := checkImages(t.Context(), docker, prj)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
}

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

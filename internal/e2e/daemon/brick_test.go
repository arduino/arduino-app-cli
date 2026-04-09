// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/store"
)

func setupTestBrick(t *testing.T) (*client.CreateAppResp, *client.ClientWithResponses) {
	httpClient := GetHttpclient(t)
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon:        f.Ptr("💻"),
			Name:        "test-app",
			Description: f.Ptr("My app description"),
		},
		func(ctx context.Context, req *http.Request) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	resp, err := httpClient.UpsertAppBrickInstanceWithResponse(
		t.Context(),
		*createResp.JSON201.Id,
		ImageClassifactionBrickID,
		client.BrickCreateUpdateRequest{Model: f.Ptr("mobilenet-image-classification")},
		func(ctx context.Context, req *http.Request) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())

	return createResp, httpClient
}

func TestBricksList(t *testing.T) {
	httpClient := GetHttpclient(t)

	response, err := httpClient.GetBricksWithResponse(t.Context(), func(ctx context.Context, req *http.Request) error { return nil })
	require.NoError(t, err)
	require.NotEmpty(t, response.JSON200.Bricks)

	staticStore := store.NewStaticStore(paths.New("testdata", "assets", config.RunnerVersion).String())
	brickIndex, err := bricksindex.Load(platform.GetPlatform(nil), staticStore.GetAssetsFolder())
	require.NoError(t, err)

	// Compare the response with the bricks index
	for _, brick := range *response.JSON200.Bricks {
		bIdx, found := brickIndex.FindBrickByID(*brick.Id)
		require.True(t, found)
		require.Equal(t, bIdx.Name, *brick.Name)
		require.Equal(t, bIdx.Description, *brick.Description)
		require.Equal(t, "Arduino", *brick.Author)
		require.Equal(t, "installed", *brick.Status)
		require.Equal(t, bIdx.RequireModel, *brick.RequireModel)
	}
}

func TestBricksDetails(t *testing.T) {
	_, httpClient := setupTestBrick(t)

	t.Run("should return 404 Not Found for an invalid brick ID", func(t *testing.T) {
		invalidBrickID := "notvalidBrickId"
		var actualBody models.ErrorResponse
		expectedDetails := fmt.Sprintf("brick with id %q not found", invalidBrickID)

		response, err := httpClient.GetBrickDetailsWithResponse(t.Context(), invalidBrickID, func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, response.StatusCode(), "status code should be 404 Not Found")

		err = json.Unmarshal(response.Body, &actualBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")

		require.Equal(t, expectedDetails, actualBody.Details, "The error detail message is not what was expected")
	})

	t.Run("should return 200 OK with full details for a valid brick ID", func(t *testing.T) {
		validBrickID := "arduino:image_classification"

		expectedUsedByApps := []client.AppReference{
			{
				Id:   f.Ptr("dXNlcjp0ZXN0LWFwcA"),
				Name: f.Ptr("test-app"),
				Icon: f.Ptr("💻"),
			},
		}

		expectedModelLiteInfo := []client.AIModel{
			{
				Id:          f.Ptr("mobilenet-image-classification"),
				Name:        f.Ptr("General purpose image classification"),
				Description: f.Ptr("General purpose image classification model based on MobileNetV2. This model is trained on the ImageNet dataset and can classify images into 1000 categories."),
			},
			{
				Id:          f.Ptr("person-classification"),
				Name:        f.Ptr("Person classification"),
				Description: f.Ptr("Person classification model based on WakeVision dataset. This model is trained to classify images into two categories: person and not-person."),
			}}

		response, err := httpClient.GetBrickDetailsWithResponse(t.Context(), validBrickID, func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode(), "status code should be 200 ok")
		require.Equal(t, "Arduino", *response.JSON200.Author)
		require.Equal(t, "installed", *response.JSON200.Status)
		require.Equal(t, "arduino:image_classification", *response.JSON200.Id)
		require.Equal(t, "Image Classification", *response.JSON200.Name)
		require.NotEmpty(t, *response.JSON200.Description, "description should not be empty")
		require.Equal(t, "video", *response.JSON200.Category)
		require.NotEmpty(t, *response.JSON200.Readme)
		require.NotNil(t, response.JSON200.UsedByApps, "UsedByApps should not be nil")
		require.Equal(t, expectedUsedByApps, *(response.JSON200.UsedByApps))
		require.NotNil(t, response.JSON200.CompatibleModels, "Models should not be nil")
		require.Equal(t, expectedModelLiteInfo, *(response.JSON200.CompatibleModels))
		require.NotNil(t, response.JSON200.ConfigVariables, "ConfigVariables should not be nil")
		// hidden variables are not returned in the details endpoint
		require.Nil(t, response.JSON200.Variables)
		require.Equal(t, []client.BrickConfigVariable{}, *(response.JSON200.ConfigVariables))
	})
}

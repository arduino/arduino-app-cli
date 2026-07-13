// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func setupTestBrick(t *testing.T) (*client.CreateAppResp, *client.ClientWithResponses) {
	httpClient := GetHttpclient(t)
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: new(true)},
		client.CreateAppRequest{
			Icon:        new("💻"),
			Name:        "test-app",
			Description: new("My app description"),
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
		client.BrickCreateUpdateRequest{Model: new("mobilenet-image-classification")},
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

	brickIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata", "assets", config.RunnerVersion))
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
				Id:   new("dXNlcjp0ZXN0LWFwcA"),
				Name: new("test-app"),
				Icon: new("💻"),
			},
		}

		expectedModelLiteInfo := []client.AIModel{
			{
				Id:          new("mobilenet-image-classification"),
				Name:        new("General purpose image classification"),
				Description: new("General purpose image classification model based on MobileNetV2. This model is trained on the ImageNet dataset and can classify images into 1000 categories."),
			},
			{
				Id:          new("person-classification"),
				Name:        new("Person classification"),
				Description: new("Person classification model based on WakeVision dataset. This model is trained to classify images into two categories: person and not-person."),
			},
			{
				Id:          new("ei:efficientnet-b4"),
				Name:        new("General purpose object classification - EfficientNet-B4"),
				Description: new("EfficientNetB4 is a machine learning model that can classify images from the Imagenet dataset. It can also be used as a backbone in building more complex models for specific use cases. This version of the model is optimized for NPU acceleration on supported devices, providing faster inference times while maintaining accuracy."),
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

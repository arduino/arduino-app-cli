// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/custommodel"
)

func TestAIModelList(t *testing.T) {

	httpClient := GetHttpclient(t)
	var allAIModelsLen int

	t.Run("should return all models when no filter is applied", func(t *testing.T) {
		response, err := httpClient.GetAIModelsWithResponse(t.Context(), nil)

		require.NoError(t, err)
		require.NotEmpty(t, response.JSON200.Models)
		allAIModelsLen = len(*response.JSON200.Models)
	})

	t.Run("should return a smaller,filtered list of models when brick filter is applied", func(t *testing.T) {
		AllModelsResponse, err := httpClient.GetAIModelsWithResponse(t.Context(), nil)
		require.NoError(t, err)
		require.NotNil(t, AllModelsResponse.JSON200)
		allAIModelsLen = len(*AllModelsResponse.JSON200.Models)

		brickId := "arduino:object_detection"
		response, err := httpClient.GetAIModelsWithResponse(t.Context(), &client.GetAIModelsParams{
			Bricks: &brickId,
		})
		require.NoError(t, err)
		require.NotEmpty(t, *response.JSON200.Models)
		require.Less(t, len(*response.JSON200.Models), allAIModelsLen)
	})

}

func TestAIModelDetails(t *testing.T) {
	customModelDir, err := paths.MkTempDir("", "custom-models")
	require.NoError(t, err)

	httpClient := GetHttpclient(t, e2e.WithCustomModelDir(customModelDir))

	aiModelsList, err := httpClient.GetAIModelsWithResponse(t.Context(), nil)
	require.NoError(t, err, "The HTTP client should not return an error for a 200 response")
	require.NotNil(t, aiModelsList.JSON200, "Setup failed: API returned a nil success body")
	require.NotEmpty(t, aiModelsList.JSON200.Models)

	expectedModel := (*aiModelsList.JSON200.Models)[0]
	require.NotNil(t, expectedModel.Id, "Setup model's ID should not be nil")
	require.NotNil(t, expectedModel.BrickIds, "Setup model's BrickId should not be nil")
	require.NotNil(t, expectedModel.Name, "Setup model's Name should not be nil")
	require.NotNil(t, expectedModel.Description, "Setup model's Description should not be nil")
	require.NotNil(t, expectedModel.Metadata, "Setup model's Metadata should not be nil")
	require.NotNil(t, expectedModel.Runner, "Setup model's Runner should not be nil")

	t.Run("should return full details for a valid model ID", func(t *testing.T) {
		// We have to add an empty editor because there is a bug that make the function panic if we pass nil
		response, err := httpClient.GetAIModelDetailsWithResponse(t.Context(), *expectedModel.Id, func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err, "The HTTP client should not return an error for a 200 response")

		modelDetails := response.JSON200

		require.NotNil(t, modelDetails.Id, "Response model's ID should not be nil")
		require.Equal(t, *expectedModel.Id, *modelDetails.Id, "ID should match")

		require.NotNil(t, modelDetails.BrickIds, "Response model's BrickId should not be nil")
		require.Equal(t, *expectedModel.BrickIds, *modelDetails.BrickIds, "BrickIds should match")

		require.NotNil(t, modelDetails.Name, "Response model's Name should not be nil")
		require.Equal(t, *expectedModel.Name, *modelDetails.Name, "Name should match")

		require.NotNil(t, modelDetails.Description, "Response model's Description should not be nil")
		require.Equal(t, *expectedModel.Description, *modelDetails.Description, "Description should match")

		require.NotNil(t, modelDetails.Metadata, "Response model's Metadata should not be nil")
		require.Equal(t, expectedModel.Metadata, modelDetails.Metadata, "Metadata should match")

		require.NotNil(t, modelDetails.Runner, "Response model's Runner should not be nil")
		require.Equal(t, *expectedModel.Runner, *modelDetails.Runner, "Runner should match")
		require.Nil(t, modelDetails.DiskUsage, "Response model's DiskUsage should be nil")

	})

	t.Run("should return full details for a valid custom model ID", func(t *testing.T) {
		_, err := custommodel.Store(customModelDir.Join("my-model"), custommodel.ModelDescriptor{
			ID:          "custom-classification-model-eim",
			Name:        "this the name of the model",
			Description: "this is the description of the model",
			Bricks: []custommodel.BrickConfig{
				{ID: "arduino:audio_classification"},
			},
		}, io.NopCloser(bytes.NewReader([]byte("some random data to create a non empty model file"))), "model.eim")
		require.NoError(t, err)

		// We have to add an empty editor because there is a bug that make the function panic if we pass nil
		response, err := httpClient.GetAIModelDetailsWithResponse(t.Context(), "custom-classification-model-eim", func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.NotNil(t, response.JSON200)

		got := response.JSON200
		require.Equal(t, &client.AIModelItem{
			Id:          f.Ptr("custom-classification-model-eim"),
			Name:        f.Ptr("this the name of the model"),
			IsBuiltin:   f.Ptr(false),
			Runner:      f.Ptr(""),
			Description: f.Ptr("this is the description of the model"),
			BrickIds:    &[]string{"arduino:audio_classification"},
			DiskUsage:   f.Ptr(222),
		}, got, "The returned model details should match the expected values")

		// TODO test metadata and model configuration contents and runner
		/*
			    require.NotNil(t, customModelDetails.Metadata, "Response model's Metadata should not be nil")
				require.Equal(t, data, customModelDetails.Metadata, "Metadata should match")

				require.NotNil(t, customModelDetails.ModelConfiguration, "Response model's ModelConfiguration should not be nil")
				require.Equal(t, expectedModel.ModelConfiguration, customModelDetails.ModelConfiguration, "ModelConfiguration should match")

				require.NotNil(t, customModelDetails.Runner, "Response model's Runner should not be nil")
				require.Equal(t, *expectedModel.Runner, *customModelDetails.Runner, "Runner should match")
		*/
	})

	t.Run("should return 404 not found for an unknown model id", func(t *testing.T) {
		unknownModelId := "invalid_model_id"
		requestEditor := func(ctx context.Context, req *http.Request) error { return nil }
		expectedDetails := fmt.Sprintf("models with id %q not found", unknownModelId)
		var actualBody models.ErrorResponse

		response, err := httpClient.GetAIModelDetailsWithResponse(context.Background(), unknownModelId, requestEditor)

		require.NoError(t, err, "The HTTP client should not return an error for a 404 response")
		require.Equal(t, http.StatusNotFound, response.StatusCode(), "Status code should be 404 Not Found")

		err = json.Unmarshal(response.Body, &actualBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")

		require.Equal(t, expectedDetails, actualBody.Details, "The error detail message is not what was expected")
	})

}

func TestAIModelDelete(t *testing.T) {
	customModelDir, err := paths.MkTempDir("", "custom-models")
	require.NoError(t, err)

	httpClient := GetHttpclient(t, e2e.WithCustomModelDir(customModelDir))

	t.Run("error on empty model id", func(t *testing.T) {
		modelId := " "
		requestEditor := func(ctx context.Context, req *http.Request) error { return nil }
		expectedDetails := "id must be set"
		var actualBody models.ErrorResponse

		response, err := httpClient.DeleteAIModelWithResponse(t.Context(), modelId, &client.DeleteAIModelParams{Force: f.Ptr(false)}, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusPreconditionFailed, response.StatusCode())
		err = json.Unmarshal(response.Body, &actualBody)
		require.NoError(t, err)
		require.Equal(t, expectedDetails, actualBody.Details)
	})

	t.Run("not found error on model not found", func(t *testing.T) {
		modelId := "invalid_model_id"
		requestEditor := func(ctx context.Context, req *http.Request) error { return nil }
		expectedDetails := fmt.Sprintf("%q: model not found", modelId)
		var actualBody models.ErrorResponse

		response, err := httpClient.DeleteAIModelWithResponse(t.Context(), modelId, &client.DeleteAIModelParams{Force: f.Ptr(false)}, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, response.StatusCode())
		err = json.Unmarshal(response.Body, &actualBody)
		require.NoError(t, err)
		require.Equal(t, expectedDetails, actualBody.Details)
	})

	t.Run("conflict error on internal model deletion", func(t *testing.T) {
		modelId := "face-detection"
		requestEditor := func(ctx context.Context, req *http.Request) error { return nil }
		expectedDetails := "cannot remove an internal model"
		var actualBody models.ErrorResponse

		response, err := httpClient.DeleteAIModelWithResponse(t.Context(), modelId, &client.DeleteAIModelParams{Force: f.Ptr(false)}, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, response.StatusCode())
		err = json.Unmarshal(response.Body, &actualBody)
		require.NoError(t, err)
		require.Equal(t, expectedDetails, actualBody.Details)
	})

	t.Run("delete a referenced model", func(t *testing.T) {
		availableModels := 0
		modelId := "my-custom-classification-model-eim"
		requestEditor := func(ctx context.Context, req *http.Request) error { return nil }
		expectedDetails := "The model is referenced by bricks belonging to the following apps: test-app-ai-model-deletion: can't delete the model"
		var actualBody models.ErrorResponse

		_, err := custommodel.Store(customModelDir.Join("my-custom-model"), custommodel.ModelDescriptor{
			ID:     modelId,
			Runner: "brick",
			Bricks: []custommodel.BrickConfig{
				{ID: "arduino:audio_classification"},
			},
		}, io.NopCloser(bytes.NewReader(nil)), "model.eim")
		require.NoError(t, err, "failed to store the model in the custom model directory")

		/* Create an app */
		appName := "test-app-ai-model-deletion"
		icon := "💻"
		createResp, err := httpClient.CreateAppWithResponse(
			t.Context(),
			&client.CreateAppParams{SkipSketch: f.Ptr(true)},
			client.CreateAppRequest{
				Icon: &icon,
				Name: appName,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, createResp.StatusCode())
		require.NotNil(t, createResp.JSON201)
		appID := createResp.JSON201.Id

		/* Check if the custom model is loaded */
		aiModelsList, err := httpClient.GetAIModelsWithResponse(t.Context(), nil)
		require.NoError(t, err, "The HTTP client should not return an error for a 200 response")
		require.NotNil(t, aiModelsList.JSON200, "Setup failed: API returned a nil success body")
		require.NotEmpty(t, aiModelsList.JSON200.Models)
		availableModels = len(*aiModelsList.JSON200.Models)

		/* Set the custom model in app.yaml */
		appUpdate, err := httpClient.UpsertAppBrickInstanceWithResponse(
			t.Context(),
			*appID,
			"arduino:audio_classification",
			client.BrickCreateUpdateRequest{Model: &modelId},
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, appUpdate.StatusCode())

		/* Delete the model, not forced */
		response, err := httpClient.DeleteAIModelWithResponse(t.Context(), modelId, &client.DeleteAIModelParams{Force: f.Ptr(false)}, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, response.StatusCode())
		err = json.Unmarshal(response.Body, &actualBody)
		require.NoError(t, err)
		require.Equal(t, expectedDetails, actualBody.Details)

		/* Delete the model, forced */
		response, err = httpClient.DeleteAIModelWithResponse(t.Context(), modelId, &client.DeleteAIModelParams{Force: f.Ptr(true)}, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, response.StatusCode())
		require.NoError(t, err)

		/* Check there is one less model available */
		aiModelsList, err = httpClient.GetAIModelsWithResponse(t.Context(), nil)
		require.NoError(t, err, "The HTTP client should not return an error for a 200 response")
		require.NotNil(t, aiModelsList.JSON200, "Setup failed: API returned a nil success body")
		require.NotEmpty(t, aiModelsList.JSON200.Models)
		require.Equal(t, availableModels-1, len(*aiModelsList.JSON200.Models))
	})
}

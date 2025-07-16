package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
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
	// setup
	httpClient := GetHttpclient(t)

	aiModelsList, err := httpClient.GetAIModelsWithResponse(t.Context(), nil)
	require.NoError(t, err, "The HTTP client should not return an error for a 200 response")
	require.NotNil(t, aiModelsList.JSON200, "Setup failed: API returned a nil success body")
	require.NotEmpty(t, aiModelsList.JSON200.Models)

	expectedModel := (*aiModelsList.JSON200.Models)[0]
	require.NotNil(t, expectedModel.Id, "Setup model's ID should not be nil")
	require.NotNil(t, expectedModel.BrickId, "Setup model's BrickId should not be nil")
	require.NotNil(t, expectedModel.Name, "Setup model's Name should not be nil")
	require.NotNil(t, expectedModel.Description, "Setup model's Description should not be nil")
	require.NotNil(t, expectedModel.Metadata, "Setup model's Metadata should not be nil")
	require.NotNil(t, expectedModel.ModelConfiguration, "Setup model's ModelConfiguration should not be nil")
	require.NotNil(t, expectedModel.Runner, "Setup model's Runner should not be nil")

	t.Run("should return full details for a valid model ID", func(t *testing.T) {
		// We have to add an empty editor because there is a bug that make the function panic if we pass nil
		response, err := httpClient.GetAIModelDetailsWithResponse(t.Context(), *expectedModel.Id, func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err, "The HTTP client should not return an error for a 200 response")

		modelDetails := response.JSON200

		require.NotNil(t, modelDetails.Id, "Response model's ID should not be nil")
		require.Equal(t, *expectedModel.Id, *modelDetails.Id, "ID should match")

		require.NotNil(t, modelDetails.BrickId, "Response model's BrickId should not be nil")
		require.Equal(t, *expectedModel.BrickId, *modelDetails.BrickId, "BrickId should match")

		require.NotNil(t, modelDetails.Name, "Response model's Name should not be nil")
		require.Equal(t, *expectedModel.Name, *modelDetails.Name, "Name should match")

		require.NotNil(t, modelDetails.Description, "Response model's Description should not be nil")
		require.Equal(t, *expectedModel.Description, *modelDetails.Description, "Description should match")

		require.NotNil(t, modelDetails.Metadata, "Response model's Metadata should not be nil")
		require.Equal(t, expectedModel.Metadata, modelDetails.Metadata, "Metadata should match")

		require.NotNil(t, modelDetails.ModelConfiguration, "Response model's ModelConfiguration should not be nil")
		require.Equal(t, expectedModel.ModelConfiguration, modelDetails.ModelConfiguration, "ModelConfiguration should match")

		require.NotNil(t, modelDetails.Runner, "Response model's Runner should not be nil")
		require.Equal(t, *expectedModel.Runner, *modelDetails.Runner, "Runner should match")

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

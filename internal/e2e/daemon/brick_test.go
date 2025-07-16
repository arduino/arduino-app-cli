package daemon

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestBricksList(t *testing.T) {

	httpClient := GetHttpclient(t)

	response, err := httpClient.GetBricksWithResponse(t.Context(), func(ctx context.Context, req *http.Request) error { return nil })
	require.NoError(t, err)
	require.NotEmpty(t, response.JSON200.Bricks)

	brickIndex, err := bricksindex.GenerateBricksIndex()
	require.NoError(t, err)

	// Compare the response with the bricks index
	for _, brick := range *response.JSON200.Bricks {
		bIdx, found := brickIndex.FindBrickByID(*brick.Id)
		require.True(t, found)
		require.Equal(t, bIdx.Name, *brick.Name)
		require.Equal(t, bIdx.Description, *brick.Description)
		require.Equal(t, "Arduino", *brick.Author)
		require.Equal(t, "installed", *brick.Status)
	}
}

func TestBricksDetails(t *testing.T) {

	httpClient := GetHttpclient(t)
	t.Run("should return 404 Not Found for an invalid brick ID", func(t *testing.T) {
		invalidBrickID := "notvalidBrickId"

		response, err := httpClient.GetBrickDetailsWithResponse(t.Context(), invalidBrickID, func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, response.StatusCode(), "status code should be 404 Not Found")
		actualBody := strings.TrimSpace(string(response.Body))
		expectedBody := "{\"details\":\"brick with id \\\"notvalidBrickId\\\" not found\"}"
		require.Equal(t, expectedBody, actualBody)
	})

	t.Run("should return 200 OK with full details for a valid brick ID", func(t *testing.T) {
		validBrickID := "arduino:image_classification"

		response, err := httpClient.GetBrickDetailsWithResponse(t.Context(), validBrickID, func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode(), "status code should be 200 ok")
		require.Equal(t, "Arduino", *response.JSON200.Author)
		require.Equal(t, "installed", *response.JSON200.Status)
		require.Equal(t, "arduino:image_classification", *response.JSON200.Id)
		require.Equal(t, "Image Classification", *response.JSON200.Name)
		require.NotEmpty(t, *response.JSON200.Description, "description should not be empty")
		require.Equal(t, "", *response.JSON200.Category)
		require.Equal(t, "/opt/models/ei/", *(*response.JSON200.Variables)["CUSTOM_MODEL_PATH"].DefaultValue)
		require.Equal(t, "path to the custom model directory", *(*response.JSON200.Variables)["CUSTOM_MODEL_PATH"].Description)
		require.Equal(t, false, *(*response.JSON200.Variables)["CUSTOM_MODEL_PATH"].Required)
		require.Equal(t, "/models/ootb/ei/mobilenet-v2-224px.eim", *(*response.JSON200.Variables)["EI_CLASSIFICATION_MODEL"].DefaultValue)
		require.Equal(t, "path to the model file", *(*response.JSON200.Variables)["EI_CLASSIFICATION_MODEL"].Description)
		require.Equal(t, false, *(*response.JSON200.Variables)["EI_CLASSIFICATION_MODEL"].Required)
		require.Equal(t, "", *response.JSON200.Readme)
		require.Nil(t, response.JSON200.UsedByApps)
	})
}

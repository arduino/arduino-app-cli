package daemon

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestAIModelList(t *testing.T) {

	httpClient := GetHttpclient(t)

	var allAIModelsLen int
	response, err := httpClient.GetAIModelsWithResponse(t.Context(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, response.JSON200.Models)
	allAIModelsLen = len(*response.JSON200.Models)

	response, err = httpClient.GetAIModelsWithResponse(t.Context(), &client.GetAIModelsParams{
		Bricks: f.Ptr("arduino:object_detection"),
	})
	require.NoError(t, err)
	require.NotEmpty(t, *response.JSON200.Models)
	require.Less(t, len(*response.JSON200.Models), allAIModelsLen)
}

func TestAIModelDetails(t *testing.T) {
	// setup
	httpClient := GetHttpclient(t)

	aiModels, err := httpClient.GetAIModelsWithResponse(t.Context(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, aiModels.JSON200.Models)

	firstAIMOdel := (*aiModels.JSON200.Models)[0]

	// We have to add an empty editor because there is a bug that make the function panic if we pass nil
	response, err := httpClient.GetAIModelDetailsWithResponse(t.Context(), *firstAIMOdel.Id, func(ctx context.Context, req *http.Request) error { return nil })
	require.NoError(t, err)
	require.NotEmpty(t, response.JSON200)
	require.NotEmpty(t, response.JSON200.Id)
	require.NotEmpty(t, response.JSON200.BrickId)
	require.NotEmpty(t, response.JSON200.Name)
	require.NotEmpty(t, response.JSON200.Description)
	require.NotEmpty(t, response.JSON200.Metadata)
	require.NotEmpty(t, response.JSON200.ModelConfiguration)
	require.NotEmpty(t, response.JSON200.Runner)
}

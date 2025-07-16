//nolint:bodyclose
package daemon

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestGetAppBrickInstances(t *testing.T) {

	httpClient := GetHttpclient(t)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon:   f.Ptr("💻"),
			Name:   "test-app",
			Bricks: &[]string{ImageClassifactionBrickID},
		},
		func(ctx context.Context, req *http.Request) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	brickInstances, err := httpClient.GetAppBrickInstancesWithResponse(t.Context(), *createResp.JSON201.Id, func(ctx context.Context, req *http.Request) error { return nil })
	require.NoError(t, err)
	require.Len(t, *brickInstances.JSON200.Bricks, 1)
	require.Equal(t, ImageClassifactionBrickID, *(*brickInstances.JSON200.Bricks)[0].Id)

	brickInstance, err := httpClient.GetAppBrickInstanceByBrickIDWithResponse(
		t.Context(),
		*createResp.JSON201.Id,
		ImageClassifactionBrickID,
		func(ctx context.Context, req *http.Request) error { return nil })
	require.NoError(t, err)
	require.NotEmpty(t, brickInstance.JSON200)
	require.Equal(t, ImageClassifactionBrickID, *brickInstance.JSON200.Id)
}

func TestUpsertAppBrickInstance(t *testing.T) {

	httpClient := GetHttpclient(t)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon: f.Ptr("💻"),
			Name: "test-app",
		},
		func(ctx context.Context, req *http.Request) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	// Create the brick instance
	resp, err := httpClient.UpsertAppBrickInstanceWithResponse(
		t.Context(),
		*createResp.JSON201.Id,
		ImageClassifactionBrickID,
		client.BrickCreateUpdateRequest{
			Model:     f.Ptr("person-classification"),
			Variables: &map[string]string{"CUSTOM_MODEL_PATH": "overidden"},
		},
		func(ctx context.Context, req *http.Request) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())

	// Verify the brick instance was updated
	brickInstance, err := httpClient.GetAppBrickInstanceByBrickIDWithResponse(
		t.Context(),
		*createResp.JSON201.Id,
		ImageClassifactionBrickID,
		func(ctx context.Context, req *http.Request) error { return nil })
	require.NoError(t, err)
	require.NotEmpty(t, brickInstance.JSON200)
	require.Equal(t, ImageClassifactionBrickID, *brickInstance.JSON200.Id)
	require.Equal(t, "overidden", (*brickInstance.JSON200.Variables)["CUSTOM_MODEL_PATH"])
	require.Equal(t, "person-classification", *brickInstance.JSON200.Model)

	t.Run("OverrideBrickInstance", func(t *testing.T) {
		resp, err := httpClient.UpsertAppBrickInstanceWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{Model: f.Ptr("mobilenet-image-classification")},
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())

		// Verify the brick instance was updated again
		brickInstance, err := httpClient.GetAppBrickInstanceByBrickIDWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.NotEmpty(t, brickInstance.JSON200)
		require.Equal(t, ImageClassifactionBrickID, *brickInstance.JSON200.Id)
		require.NotEqual(t, "overidden", (*brickInstance.JSON200.Variables)["CUSTOM_MODEL_PATH"])
		require.Equal(t, "mobilenet-image-classification", *brickInstance.JSON200.Model)
	})

	t.Run("WrongModelFails", func(t *testing.T) {
		resp, err := httpClient.UpsertAppBrickInstance(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{Model: f.Ptr("non-existent-model")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
	t.Run("NotExistingBrickIDFails", func(t *testing.T) {
		resp, err := httpClient.UpsertAppBrickInstance(
			t.Context(),
			*createResp.JSON201.Id,
			"invalid-brick-id",
			client.BrickCreateUpdateRequest{},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
	t.Run("NotExistingVariableFails", func(t *testing.T) {
		resp, err := httpClient.UpsertAppBrickInstance(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{
				Variables: &map[string]string{"NOT_EXISTING": "value"},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestUpdateAppBrickInstance(t *testing.T) {

	httpClient := GetHttpclient(t)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon:   f.Ptr("💻"),
			Name:   "test-app",
			Bricks: &[]string{ImageClassifactionBrickID},
		},
		func(ctx context.Context, req *http.Request) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	t.Run("UpdateAppBrickInstance", func(t *testing.T) {
		resp, err := httpClient.UpdateAppBrickInstanceWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{
				Model:     f.Ptr("person-classification"),
				Variables: &map[string]string{"CUSTOM_MODEL_PATH": "overidden"},
			},
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())

		// Verify the brick instance was updated
		brickInstance, err := httpClient.GetAppBrickInstanceByBrickIDWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.NotEmpty(t, brickInstance.JSON200)
		require.Equal(t, ImageClassifactionBrickID, *brickInstance.JSON200.Id)
		require.Equal(t, "overidden", (*brickInstance.JSON200.Variables)["CUSTOM_MODEL_PATH"])
		require.Equal(t, "person-classification", *brickInstance.JSON200.Model)
	})
	t.Run("UpdateOnlyModel", func(t *testing.T) {
		resp, err := httpClient.UpdateAppBrickInstanceWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{Model: f.Ptr("mobilenet-image-classification")},
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())

		// Verify the brick instance was updated again
		brickInstance, err := httpClient.GetAppBrickInstanceByBrickIDWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.NotEmpty(t, brickInstance.JSON200)
		require.Equal(t, ImageClassifactionBrickID, *brickInstance.JSON200.Id)
		require.Equal(t, "mobilenet-image-classification", *brickInstance.JSON200.Model)
	})

	t.Run("UpdateWithWrongModelFails", func(t *testing.T) {
		resp, err := httpClient.UpdateAppBrickInstance(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{Model: f.Ptr("non-existent-model")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
	t.Run("UpdateWithNotExistingVariableFails", func(t *testing.T) {
		resp, err := httpClient.UpdateAppBrickInstance(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{
				Variables: &map[string]string{"NOT_EXISTING": "value"},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestDeleteAppBrickInstance(t *testing.T) {

	httpClient := GetHttpclient(t)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon:   f.Ptr("💻"),
			Name:   "test-app",
			Bricks: &[]string{ImageClassifactionBrickID},
		},
		func(ctx context.Context, req *http.Request) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	// Delete the brick instance
	resp, err := httpClient.DeleteAppBrickInstance(
		t.Context(),
		*createResp.JSON201.Id,
		ImageClassifactionBrickID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify the brick instance was deleted
	brickInstances, err := httpClient.GetAppBrickInstancesWithResponse(t.Context(), *createResp.JSON201.Id, func(ctx context.Context, req *http.Request) error { return nil })
	require.NoError(t, err)
	require.Len(t, *brickInstances.JSON200.Bricks, 0)
}

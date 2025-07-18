//nolint:bodyclose
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestGetAppBrickInstances(t *testing.T) {

	httpClient := GetHttpclient(t)
	var actualBody models.ErrorResponse

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

	t.Run("GetAppBrickInstances_Success", func(t *testing.T) {
		brickInstances, err := httpClient.GetAppBrickInstancesWithResponse(t.Context(), *createResp.JSON201.Id, func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.Len(t, *brickInstances.JSON200.Bricks, 1)
		require.Equal(t, ImageClassifactionBrickID, *(*brickInstances.JSON200.Bricks)[0].Id)

	})

	t.Run("GetAppBrickInstances_InvalidAppID_Fail", func(t *testing.T) {

		brickInstances, err := httpClient.GetAppBrickInstancesWithResponse(t.Context(), malformedAppId, func(ctx context.Context, req *http.Request) error { return nil })

		require.NoError(t, err, "The HTTP client should not return an error for a 412 response")
		require.Equal(t, http.StatusPreconditionFailed, brickInstances.StatusCode(), "Status code should be 412 precondition failed")

		err = json.Unmarshal(brickInstances.Body, &actualBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, expectedDetailsAppInvalidAppId, actualBody.Details, "The error detail message is not what was expected")

	})

	t.Run("GetAppBrickInstances_NoExistingApp_Fail", func(t *testing.T) {
		brickInstances, err := httpClient.GetAppBrickInstancesWithResponse(t.Context(), noExistingApp, func(ctx context.Context, req *http.Request) error { return nil })

		require.NoError(t, err, "The HTTP client should not return an error for a 500 response")
		require.Equal(t, http.StatusInternalServerError, brickInstances.StatusCode(), "Status code should be 500 internal server error")

		err = json.Unmarshal(brickInstances.Body, &actualBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, expectedDetailsAppNotfound, actualBody.Details, "The error detail message is not what was expected")

	})
}

func TestGetAppBrickInstanceById(t *testing.T) {

	httpClient := GetHttpclient(t)

	var actualBody models.ErrorResponse

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

	t.Run("GetAppBrickInstanceByBrickID_Success", func(t *testing.T) {
		brickInstance, err := httpClient.GetAppBrickInstanceByBrickIDWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			func(ctx context.Context, req *http.Request) error { return nil })
		require.NoError(t, err)
		require.NotEmpty(t, brickInstance.JSON200)
		require.Equal(t, ImageClassifactionBrickID, *brickInstance.JSON200.Id)
	})

	t.Run("GetAppBrickInstanceByBrickID_InvalidAppID_Fails", func(t *testing.T) {

		brickInstance, err := httpClient.GetAppBrickInstanceByBrickIDWithResponse(
			t.Context(),
			malformedAppId,
			ImageClassifactionBrickID,
			func(ctx context.Context, req *http.Request) error { return nil })

		require.NoError(t, err, "The HTTP client should not return an error for a 412 response")
		require.Equal(t, http.StatusPreconditionFailed, brickInstance.StatusCode(), "Status code should be 412 precondition failed")

		err = json.Unmarshal(brickInstance.Body, &actualBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")

		require.Equal(t, expectedDetailsAppInvalidAppId, actualBody.Details, "The error detail message is not what was expected")
	})

	t.Run("GetAppBrickInstanceByBrickID_NoExistingApp_Fail", func(t *testing.T) {

		brickInstance, err := httpClient.GetAppBrickInstanceByBrickIDWithResponse(
			t.Context(),
			noExistingApp,
			ImageClassifactionBrickID,
			func(ctx context.Context, req *http.Request) error { return nil })

		require.NoError(t, err, "The HTTP client should not return an error for a 500 response")
		require.Equal(t, http.StatusInternalServerError, brickInstance.StatusCode(), "Status code should be 500 internal server error")

		err = json.Unmarshal(brickInstance.Body, &actualBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")

		require.Equal(t, expectedDetailsAppNotfound, actualBody.Details, "The error detail message is not what was expected")
	})

}

func TestUpsertAppBrickInstance(t *testing.T) {
	var actualResponseBody models.ErrorResponse
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
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "error while creating or updating brick", actualResponseBody.Details, "The error detail message is not what was expected")

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
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "error while creating or updating brick", actualResponseBody.Details, "The error detail message is not what was expected")
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
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "error while creating or updating brick", actualResponseBody.Details, "The error detail message is not what was expected")
	})
	t.Run("NotValidAppDdFails", func(t *testing.T) {
		resp, err := httpClient.UpsertAppBrickInstance(
			t.Context(),
			malformedAppId,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{
				Variables: &map[string]string{"NOT_EXISTING": "value"},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, expectedDetailsAppInvalidAppId, actualResponseBody.Details, "The error detail message is not what was expected")
	})

	t.Run("NotExistingAppFails", func(t *testing.T) {
		resp, err := httpClient.UpsertAppBrickInstance(
			t.Context(),
			noExistingApp,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{
				Variables: &map[string]string{"NOT_EXISTING": "value"},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, expectedDetailsAppNotfound, actualResponseBody.Details, "The error detail message is not what was expected")
	})
}

func TestUpdateAppBrickInstance(t *testing.T) {
	var actualResponseBody models.ErrorResponse
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
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "unable to update the brick", actualResponseBody.Details, "The error detail message is not what was expected")

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

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "unable to update the brick", actualResponseBody.Details, "The error detail message is not what was expected")
	})

	t.Run("UpdateWithNotExistingAppFails", func(t *testing.T) {
		resp, err := httpClient.UpdateAppBrickInstance(
			t.Context(),
			noExistingApp,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{
				Variables: &map[string]string{"NOT_EXISTING": "value"},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, expectedDetailsAppNotfound, actualResponseBody.Details, "The error detail message is not what was expected")
	})

	t.Run("UpdateWithNotValidAppIdFails", func(t *testing.T) {
		resp, err := httpClient.UpdateAppBrickInstance(
			t.Context(),
			malformedAppId,
			ImageClassifactionBrickID,
			client.BrickCreateUpdateRequest{
				Variables: &map[string]string{"NOT_EXISTING": "value"},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, expectedDetailsAppInvalidAppId, actualResponseBody.Details, "The error detail message is not what was expected")
	})

	t.Run("UpdateWithWrongRequestBodyFails", func(t *testing.T) {

		setInvalidBodyEditor := func(ctx context.Context, req *http.Request) error {
			invalidBodyString := `{"variables": "questo non è un oggetto"}`
			req.Body = io.NopCloser(strings.NewReader(invalidBodyString))
			req.ContentLength = int64(len(invalidBodyString))
			req.Header.Set("Content-Type", "application/json")

			return nil
		}

		resp, err := httpClient.UpdateAppBrickInstance(
			t.Context(),
			*createResp.JSON201.Id,
			ImageClassifactionBrickID,
			client.UpdateAppBrickInstanceJSONRequestBody{}, // request body will be overwitten by the next param
			setInvalidBodyEditor,
		)

		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		fmt.Println("holaaaa " + string(body))
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "invalid request body", actualResponseBody.Details, "The error detail message is not what was expected")

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

	t.Run("DeleteAppBrickInstance_NoExistingAppId_fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.DeleteAppBrickInstance(
			t.Context(),
			noExistingApp,
			ImageClassifactionBrickID,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, expectedDetailsAppNotfound, actualResponseBody.Details, "The error detail message is not what was expected")

	})

	t.Run("DeleteAppBrickInstance_InvalidAppId_fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.DeleteAppBrickInstance(
			t.Context(),
			malformedAppId,
			ImageClassifactionBrickID,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, expectedDetailsAppInvalidAppId, actualResponseBody.Details, "The error detail message is not what was expected")

	})

	// Delete the brick instance
	t.Run("DeleteAppBrickInstance_Success", func(t *testing.T) {
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
	})

}

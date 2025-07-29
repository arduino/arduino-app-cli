package daemon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestCreateApp(t *testing.T) {
	httpClient := GetHttpclient(t)

	defaultRequestBody := client.CreateAppRequest{
		Icon: f.Ptr("🌎"),
		Name: "HelloWorld",
	}

	testCases := []struct {
		name                 string
		parameters           client.CreateAppParams
		body                 client.CreateAppRequest
		expectedStatusCode   int
		expectedErrorDetails *string
	}{
		{
			name: "should return 201 Created on first successful creation",
			parameters: client.CreateAppParams{
				SkipPython: f.Ptr(false),
				SkipSketch: f.Ptr(false),
			},
			body:               defaultRequestBody,
			expectedStatusCode: http.StatusCreated,
		},
		{
			name: "should return 409 Conflict when creating a duplicate app",
			parameters: client.CreateAppParams{
				SkipPython: f.Ptr(false),
				SkipSketch: f.Ptr(false),
			},
			body:                 defaultRequestBody,
			expectedStatusCode:   http.StatusConflict,
			expectedErrorDetails: f.Ptr("app already exists"),
		},
		{
			name: "should return 201 Created on successful creation with skip_python",
			parameters: client.CreateAppParams{
				SkipPython: f.Ptr(true),
				SkipSketch: f.Ptr(false),
			},
			body: client.CreateAppRequest{
				Icon: f.Ptr("🌎"),
				Name: "HelloWorld_2",
			},
			expectedStatusCode: http.StatusCreated,
		},
		{
			name: "should return 201 Created on successful creation with skip_sketch",
			parameters: client.CreateAppParams{
				SkipPython: f.Ptr(false),
				SkipSketch: f.Ptr(true),
			},
			body: client.CreateAppRequest{
				Icon: f.Ptr("🌎"),
				Name: "HelloWorld_3",
			},
			expectedStatusCode: http.StatusCreated,
		},
		{
			name: "should return 400 Bad Request when creating an app with both filters set to true",
			parameters: client.CreateAppParams{
				SkipPython: f.Ptr(true),
				SkipSketch: f.Ptr(true),
			},
			body:                 defaultRequestBody,
			expectedStatusCode:   http.StatusBadRequest,
			expectedErrorDetails: f.Ptr("cannot skip both python and sketch"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := httpClient.CreateApp(t.Context(), &tc.parameters, tc.body)
			require.NoError(t, err)
			defer r.Body.Close()

			require.Equal(t, tc.expectedStatusCode, r.StatusCode)

			if tc.expectedErrorDetails != nil {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var actualErrorResponse models.ErrorResponse
				err = json.Unmarshal(body, &actualErrorResponse)
				require.NoError(t, err, "Failed to unmarshal JSON error response")

				require.Equal(t, *tc.expectedErrorDetails, actualErrorResponse.Details, "The error detail message is not what was expected")
			}
		})
	}
}

func TestEditApp(t *testing.T) {
	httpClient := GetHttpclient(t)

	appName := "test-app-to-edit"
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon: f.Ptr("💻"),
			Name: appName,
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	validAppId := *createResp.JSON201.Id

	t.Run("EditAllFields_Success", func(t *testing.T) {
		renamedApp := appName + "-renamed"
		modifedIcon := "🌟"
		editResp, err := httpClient.EditAppWithResponse(
			t.Context(),
			validAppId,
			client.EditRequest{
				Description: f.Ptr("new-description"),
				Icon:        f.Ptr(modifedIcon),
				Name:        f.Ptr(renamedApp),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, editResp.StatusCode())
		require.NotNil(t, editResp.JSON200)
		require.NotNil(t, editResp.JSON200.Id)
		detailsResp, err := httpClient.GetAppDetailsWithResponse(t.Context(), editResp.JSON200.Id)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, detailsResp.StatusCode())
		require.Equal(t, renamedApp, detailsResp.JSON200.Name)
		require.Equal(t, modifedIcon, *detailsResp.JSON200.Icon)
	})

	t.Run("InvalidAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		editResp, err := httpClient.EditApp(
			t.Context(),
			malformedAppId,
			client.EditRequest{Name: f.Ptr("any-name")},
		)
		require.NoError(t, err)
		defer editResp.Body.Close()

		require.Equal(t, http.StatusPreconditionFailed, editResp.StatusCode)
		body, err := io.ReadAll(editResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "invalid id", actualResponseBody.Details)
	})
	t.Run("NonExistentAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		editResp, err := httpClient.EditApp(
			t.Context(),
			"dXNlcjp0ZXN0LWFwcAw",
			client.EditRequest{
				Description: f.Ptr("new-description"),
				Icon:        f.Ptr("🌟"),
				Name:        f.Ptr("new name"),
			},
		)
		require.NoError(t, err)
		defer editResp.Body.Close()

		require.Equal(t, http.StatusInternalServerError, editResp.StatusCode)
		body, err := io.ReadAll(editResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "unable to find the app", actualResponseBody.Details)
	})

	t.Run("InvalidRequestBody_Fail", func(t *testing.T) {
		createResp, err := httpClient.CreateAppWithResponse(
			t.Context(),
			&client.CreateAppParams{SkipSketch: f.Ptr(true)},
			client.CreateAppRequest{
				Icon: f.Ptr("💻"),
				Name: "new-valid-app",
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, createResp.StatusCode())
		require.NotNil(t, createResp.JSON201)

		validAppId := *createResp.JSON201.Id

		var actualResponseBody models.ErrorResponse
		malformedBody := `{"name": "test" "icon": "💻"}`
		editResp, err := httpClient.EditAppWithBody(
			t.Context(),
			validAppId,
			"application/json",
			strings.NewReader(malformedBody),
		)
		require.NoError(t, err)
		defer editResp.Body.Close()

		require.Equal(t, http.StatusBadRequest, editResp.StatusCode)
		body, err := io.ReadAll(editResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "invalid request", actualResponseBody.Details)
	})
}

func TestDeleteApp(t *testing.T) {
	httpClient := GetHttpclient(t)

	appToDeleteName := "app-to-be-deleted"
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon: f.Ptr("🗑️"),
			Name: appToDeleteName,
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	appToDeleteId := *createResp.JSON201.Id

	t.Run("DeleteApp_Success", func(t *testing.T) {

		deleteResp, err := httpClient.DeleteApp(t.Context(), appToDeleteId)
		require.NoError(t, err)
		defer deleteResp.Body.Close()
		require.Equal(t, http.StatusOK, deleteResp.StatusCode)

		getResp, err := httpClient.GetAppDetails(t.Context(), appToDeleteId)
		require.NoError(t, err)
		defer getResp.Body.Close()

		require.Equal(t, http.StatusInternalServerError, getResp.StatusCode)

		var actualResponseBody models.ErrorResponse
		body, err := io.ReadAll(getResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "unable to find the app", actualResponseBody.Details)
	})

	t.Run("InvalidAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		deleteResp, err := httpClient.DeleteApp(t.Context(), malformedAppId)
		require.NoError(t, err)
		defer deleteResp.Body.Close()

		require.Equal(t, http.StatusPreconditionFailed, deleteResp.StatusCode)
		body, err := io.ReadAll(deleteResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "invalid id", actualResponseBody.Details)
	})

	t.Run("DeletingExampleApp_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		deleteResp, err := httpClient.DeleteApp(t.Context(), noExisitingExample)
		require.NoError(t, err)
		defer deleteResp.Body.Close()

		require.Equal(t, http.StatusBadRequest, deleteResp.StatusCode)
		body, err := io.ReadAll(deleteResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "cannot delete example", actualResponseBody.Details)
	})

	t.Run("NonExistentAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		deleteResp, err := httpClient.DeleteApp(t.Context(), noExistingApp)
		require.NoError(t, err)
		defer deleteResp.Body.Close()

		require.Equal(t, http.StatusInternalServerError, deleteResp.StatusCode)
		body, err := io.ReadAll(deleteResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "unable to find the app", actualResponseBody.Details)
	})
}

func TestAppStart(t *testing.T) {
	httpClient := GetHttpclient(t)

	t.Run("InvalidAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.StartApp(t.Context(), malformedAppId)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "invalid id", actualResponseBody.Details)
	})

	t.Run("NonExistentAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.StartApp(t.Context(), noExistingApp)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "unable to find the app", actualResponseBody.Details)
	})
}

func TestAppStop(t *testing.T) {
	httpClient := GetHttpclient(t)

	t.Run("InvalidAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.StopApp(t.Context(), malformedAppId)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "invalid id", actualResponseBody.Details)
	})

	t.Run("NonExistentAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.StopApp(t.Context(), noExistingApp)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "unable to find the app", actualResponseBody.Details)
	})
}

func TestAppClone(t *testing.T) {
	httpClient := GetHttpclient(t)

	sourceAppResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{},
		client.CreateAppRequest{
			Icon: f.Ptr("📄"),
			Name: "source-app",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, sourceAppResp.StatusCode())
	sourceAppId := *sourceAppResp.JSON201.Id

	conflictAppResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{},
		client.CreateAppRequest{
			Icon: f.Ptr("🚫"),
			Name: "existing-clone-name",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, conflictAppResp.StatusCode())

	t.Run("CloneApp_Success", func(t *testing.T) {
		newCloneName := "my-awesome-clone"
		newCloneIcon := "✨"

		cloneResp, err := httpClient.CloneAppWithResponse(
			t.Context(),
			sourceAppId,
			client.CloneRequest{
				Name: f.Ptr(newCloneName),
				Icon: f.Ptr(newCloneIcon),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, cloneResp.StatusCode())
		require.NotNil(t, cloneResp.JSON201)

		clonedApp := cloneResp.JSON201
		require.NotEqual(t, sourceAppId, *clonedApp.Id)
	})

	t.Run("CloneApp_Success_WithDefaultName", func(t *testing.T) {
		cloneResp, err := httpClient.CloneAppWithResponse(t.Context(), sourceAppId, client.CloneRequest{})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, cloneResp.StatusCode())
		require.NotNil(t, cloneResp.JSON201)
	})

	t.Run("InvalidSourceId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.CloneApp(t.Context(), malformedAppId, client.CloneRequest{})
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "invalid id", actualResponseBody.Details)
	})

	t.Run("NonExistentSourceId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.CloneApp(t.Context(), noExistingApp, client.CloneRequest{})
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "app not found", actualResponseBody.Details)
	})

	t.Run("NameConflict_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.CloneApp(
			t.Context(),
			sourceAppId,
			client.CloneRequest{Name: f.Ptr("existing-clone-name")},
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusConflict, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "app already exists", actualResponseBody.Details)
	})

	t.Run("InvalidRequestBody_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		malformedBody := `{"name": "test",, "icon": "invalid"}`

		resp, err := httpClient.CloneAppWithBody(
			t.Context(),
			sourceAppId,
			"application/json",
			strings.NewReader(malformedBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "unable to decode app clone request", actualResponseBody.Details)
	})
}

func TestAppLogs(t *testing.T) {
	httpClient := GetHttpclient(t)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{},
		client.CreateAppRequest{
			Icon: f.Ptr("📜"),
			Name: "app-with-logs",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	appWithLogsId := *createResp.JSON201.Id

	startResp, err := httpClient.StartApp(t.Context(), appWithLogsId)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, startResp.Body)
	require.NoError(t, err, "Failed to unmarshal the JSON error response body")
	startResp.Body.Close()
	require.Equal(t, http.StatusOK, startResp.StatusCode)

	t.Run("InvalidAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.GetAppLogs(t.Context(), malformedAppId, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "invalid id", actualResponseBody.Details)
	})

	t.Run("NonExistentAppId_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.GetAppLogs(t.Context(), noExistingApp, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "unable to find the app", actualResponseBody.Details)
	})

	t.Run("InvalidTailValue_Fail", func(t *testing.T) {
		var actualResponseBody models.ErrorResponse
		resp, err := httpClient.GetAppLogs(t.Context(), appWithLogsId, &client.GetAppLogsParams{Tail: f.Ptr(-4)})
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err, "Failed to unmarshal the JSON error response body")
		require.Equal(t, "invalid tail value", actualResponseBody.Details)
	})
	// find a way to test 400 invalid tail value: client generated code is type safe, so an invalid value can't be sent
}

func TestAppDetails(t *testing.T) {
	httpClient := GetHttpclient(t)

	appName := "test-app-details"
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon: f.Ptr("💻"),
			Name: appName,
		},
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

	t.Run("DetailsOfApp", func(t *testing.T) {
		appID := createResp.JSON201.Id
		detailsResp, err := httpClient.GetAppDetailsWithResponse(t.Context(), *appID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, detailsResp.StatusCode())
		require.Equal(t, *appID, detailsResp.JSON200.Id)
		require.Equal(t, appName, detailsResp.JSON200.Name)
		require.Equal(t, "💻", *detailsResp.JSON200.Icon)
		require.Len(t, *detailsResp.JSON200.Bricks, 1)
		require.Equal(t,
			client.AppDetailedBrick{
				Id:       ImageClassifactionBrickID,
				Name:     "Image Classification",
				Category: f.Ptr("video"),
			},
			(*detailsResp.JSON200.Bricks)[0],
		)
		require.False(t, *detailsResp.JSON200.Example)
		require.False(t, *detailsResp.JSON200.Default)
		require.Equal(t, client.Stopped, detailsResp.JSON200.Status)
		require.NotEmpty(t, detailsResp.JSON200.Path)
	})
}

func TestAppPorts(t *testing.T) {
	httpClient := GetHttpclient(t)

	t.Run("GetAppPorts_Success", func(t *testing.T) {

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

		respBrick, err := httpClient.UpsertAppBrickInstanceWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			StreamLitUi,
			client.BrickCreateUpdateRequest{},
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, respBrick.StatusCode())

		resp, err := httpClient.GetAppPorts(
			t.Context(),
			*createResp.JSON201.Id,
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var portsResponse client.AppPortResponse
		err = json.NewDecoder(resp.Body).Decode(&portsResponse)
		require.NoError(t, err)
		require.NotEmpty(t, portsResponse.Ports)
		ports := *portsResponse.Ports
		require.Len(t, ports, 1)
		require.Equal(t, "7000", *ports[0].Port)
		require.Equal(t, "arduino:streamlit_ui", *ports[0].Source)
		require.Equal(t, "webview", *ports[0].ServiceName)

	})

	t.Run("GetAppPortsEmpty_Success", func(t *testing.T) {

		createResp, err := httpClient.CreateAppWithResponse(
			t.Context(),
			&client.CreateAppParams{SkipSketch: f.Ptr(true)},
			client.CreateAppRequest{
				Icon: f.Ptr("💻"),
				Name: "test-app-2",
			},
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, createResp.StatusCode())
		require.NotNil(t, createResp.JSON201)

		resp, err := httpClient.GetAppPorts(
			t.Context(),
			*createResp.JSON201.Id,
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var portsResponse client.AppPortResponse
		err = json.NewDecoder(resp.Body).Decode(&portsResponse)
		require.NoError(t, err)
		require.Empty(t, portsResponse.Ports)

	})

	t.Run("GetAppPortsNoexistingApp_FAil", func(t *testing.T) {

		resp, err := httpClient.GetAppPorts(
			t.Context(),
			noExistingApp,
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		var acturalResp models.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&acturalResp)
		require.NoError(t, err)
		require.Equal(t, "unable to find the app", acturalResp.Details, "The error detail message is not what was expected")

	})
	t.Run("GetAppPortsInvalidAppId", func(t *testing.T) {

		resp, err := httpClient.GetAppPorts(
			t.Context(),
			malformedAppId,
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
		var acturalResp models.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&acturalResp)
		require.NoError(t, err)
		require.Equal(t, "invalid id", acturalResp.Details, "The error detail message is not what was expected")

	})
}

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
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/api/handlers"
	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestApps(t *testing.T) {
	httpClient := GetHttpclient(t)

	appName := "test-app-details"
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
	fmt.Println(*createResp.JSON201.Id)
	fmt.Println(string(f.Must(base64.StdEncoding.DecodeString(*createResp.JSON201.Id))))
	appID := createResp.JSON201.Id

	t.Run("ok", func(t *testing.T) {
		appsResp, err := httpClient.GetAppsWithResponse(t.Context(), &client.GetAppsParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, appsResp.StatusCode())

		require.NotNil(t, appsResp.JSON200.Apps)
		require.Len(t, *appsResp.JSON200.Apps, 1)

		require.Equal(t, *appID, *(*appsResp.JSON200.Apps)[0].Id)
		require.Equal(t, appName, *(*appsResp.JSON200.Apps)[0].Name)
		require.Equal(t, icon, *(*appsResp.JSON200.Apps)[0].Icon)
		require.Equal(t, false, *(*appsResp.JSON200.Apps)[0].Example)
		require.Equal(t, false, *(*appsResp.JSON200.Apps)[0].Default)
		require.Equal(t, "", *(*appsResp.JSON200.Apps)[0].Description)

		require.Nil(t, appsResp.JSON200.BrokenApps)
	})
}

func TestCreateApp(t *testing.T) {
	httpClient := GetHttpclient(t)

	defaultRequestBody := client.CreateAppRequest{
		Icon:        f.Ptr("🌎"),
		Name:        "HelloWorld",
		Description: f.Ptr("My HelloWorld description"),
	}

	testCases := []struct {
		name                 string
		parameters           client.CreateAppParams
		body                 client.CreateAppRequest
		expectedStatusCode   int
		expectedErrorDetails *string
	}{
		{
			name: "should return 400 bad request when icon is not a single emoji",
			parameters: client.CreateAppParams{
				SkipSketch: f.Ptr(false),
			},
			body: client.CreateAppRequest{
				Icon:        f.Ptr("invalid-icon"),
				Name:        "HelloWorld-2",
				Description: f.Ptr("My HelloWorld description"),
			},
			expectedStatusCode:   http.StatusBadRequest,
			expectedErrorDetails: f.Ptr("invalid app: icon \"invalid-icon\" is not a valid single emoji"),
		},
		{
			name: "should create app successfully when icon is empty",
			parameters: client.CreateAppParams{
				SkipSketch: f.Ptr(false),
			},
			body: client.CreateAppRequest{
				Icon:        nil,
				Name:        "HelloWorld-2",
				Description: f.Ptr("My HelloWorld description"),
			},
			expectedStatusCode: http.StatusCreated,
			//expectedErrorDetails: f.Ptr("invalid app: icon cannot be empty"),
		},
		{
			name: "should return 201 Created on first successful creation",
			parameters: client.CreateAppParams{
				SkipSketch: f.Ptr(false),
			},
			body:               defaultRequestBody,
			expectedStatusCode: http.StatusCreated,
		},
		{
			name: "should return 409 Conflict when creating a duplicate app",
			parameters: client.CreateAppParams{
				SkipSketch: f.Ptr(false),
			},
			body:                 defaultRequestBody,
			expectedStatusCode:   http.StatusConflict,
			expectedErrorDetails: f.Ptr("app already exists"),
		},
		{
			name: "should return 201 Created on successful creation with skip_sketch",
			parameters: client.CreateAppParams{
				SkipSketch: f.Ptr(true),
			},
			body: client.CreateAppRequest{
				Icon:        f.Ptr("🌎"),
				Name:        "HelloWorld_3",
				Description: f.Ptr("My HelloWorld_3 description"),
			},
			expectedStatusCode: http.StatusCreated,
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
func TestCreateAndVerifyAppDetails(t *testing.T) {
	httpClient := GetHttpclient(t)
	appToCreate := client.CreateAppRequest{
		Icon:        f.Ptr("🧪"),
		Name:        "test-app-for-verification",
		Description: f.Ptr("A description for the verification test."),
	}

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		appToCreate,
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201, "The creation response body should not be nil")
	require.NotNil(t, createResp.JSON201.Id, "The created app ID should not be nil")

	createdAppId := *createResp.JSON201.Id

	detailsResp, err := httpClient.GetAppDetailsWithResponse(t.Context(), createdAppId)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, detailsResp.StatusCode())
	require.NotNil(t, detailsResp.JSON200, "The get details response body should not be nil")

	retrievedApp := detailsResp.JSON200

	require.Equal(t, createdAppId, retrievedApp.Id)
	require.Equal(t, appToCreate.Name, retrievedApp.Name)
	require.Equal(t, *appToCreate.Icon, *retrievedApp.Icon)
	require.Equal(t, *appToCreate.Description, *retrievedApp.Description)

	require.False(t, *retrievedApp.Example, "A new app should not be an 'example'")
	require.False(t, *retrievedApp.Default, "A new app should not be 'default'")
	require.Equal(t, client.Uninitialized, retrievedApp.Status, "The initial status of a new app should be 'initialized'")
	require.Empty(t, retrievedApp.Bricks, "A new app should not have 'bricks'")
	require.NotEmpty(t, retrievedApp.Path, "The app path should not be empty")
}

func TestEditApp(t *testing.T) {
	httpClient := GetHttpclient(t)

	appName := "test-app-to-edit"
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon:        f.Ptr("💻"),
			Name:        appName,
			Description: f.Ptr("My app description"),
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
	t.Run("RequestEmptyIcon_Success", func(t *testing.T) {
		createResp, err := httpClient.CreateAppWithResponse(
			t.Context(),
			&client.CreateAppParams{SkipSketch: f.Ptr(true)},
			client.CreateAppRequest{
				Icon:        f.Ptr("💻"),
				Name:        "new-valid-app-1",
				Description: f.Ptr("My app description"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, createResp.StatusCode())
		require.NotNil(t, createResp.JSON201)

		validAppId := *createResp.JSON201.Id

		invalidIconBody := `{"icon": "","description": "modified", "example": false,"default": false}`
		editResp, err := httpClient.EditAppWithBody(
			t.Context(),
			validAppId,
			"application/json",
			strings.NewReader(invalidIconBody),
		)
		require.NoError(t, err)
		defer editResp.Body.Close()
		require.Equal(t, http.StatusOK, editResp.StatusCode)
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

	t.Run("InvalidRequestSintaxBody_Fail", func(t *testing.T) {
		createResp, err := httpClient.CreateAppWithResponse(
			t.Context(),
			&client.CreateAppParams{SkipSketch: f.Ptr(true)},
			client.CreateAppRequest{
				Icon:        f.Ptr("💻"),
				Name:        "new-valid-app",
				Description: f.Ptr("My app description"),
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
	t.Run("InvalidRequestIcon_Fail", func(t *testing.T) {
		createResp, err := httpClient.CreateAppWithResponse(
			t.Context(),
			&client.CreateAppParams{SkipSketch: f.Ptr(true)},
			client.CreateAppRequest{
				Icon:        f.Ptr("💻"),
				Name:        "new-valid-app-2",
				Description: f.Ptr("My app description"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, createResp.StatusCode())
		require.NotNil(t, createResp.JSON201)

		validAppId := *createResp.JSON201.Id

		var actualResponseBody models.ErrorResponse
		invalidIconBody := `{"name": "test", "icon": "💻 invalid"}`
		editResp, err := httpClient.EditAppWithBody(
			t.Context(),
			validAppId,
			"application/json",
			strings.NewReader(invalidIconBody),
		)
		require.NoError(t, err)
		defer editResp.Body.Close()

		require.Equal(t, http.StatusBadRequest, editResp.StatusCode)
		body, err := io.ReadAll(editResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "invalid app: icon \"💻 invalid\" is not a valid single emoji", actualResponseBody.Details)
	})
}

func TestDeleteApp(t *testing.T) {
	httpClient := GetHttpclient(t)

	appToDeleteName := "app-to-be-deleted"
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon:        f.Ptr("🗑️"),
			Name:        appToDeleteName,
			Description: f.Ptr("My app description"),
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
		deleteResp, err := httpClient.DeleteApp(t.Context(), "ZXhhbXBsZXM6anVzdGJsaW5f")
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
			Icon:        f.Ptr("📄"),
			Name:        "source-app",
			Description: f.Ptr("My app description"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, sourceAppResp.StatusCode())
	sourceAppId := *sourceAppResp.JSON201.Id

	conflictAppResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{},
		client.CreateAppRequest{
			Icon:        f.Ptr("🚫"),
			Name:        "existing-clone-name",
			Description: f.Ptr("My app description"),
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
			Icon:        f.Ptr("📜"),
			Name:        "app-with-logs",
			Description: f.Ptr("My app description"),
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
			Icon:        f.Ptr("💻"),
			Name:        appName,
			Description: f.Ptr("My app description"),
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
		require.Equal(t, "My app description", *detailsResp.JSON200.Description)
		require.Len(t, *detailsResp.JSON200.Bricks, 1)
		require.Equal(t,
			client.AppDetailedBrick{
				Id:           ImageClassifactionBrickID,
				Name:         "Image Classification",
				Category:     f.Ptr("video"),
				RequireModel: f.Ptr(true),
			},
			(*detailsResp.JSON200.Bricks)[0],
		)
		require.False(t, *detailsResp.JSON200.Example)
		require.False(t, *detailsResp.JSON200.Default)
		require.Equal(t, client.Uninitialized, detailsResp.JSON200.Status)
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
				Icon:        f.Ptr("💻"),
				Name:        "test-app",
				Description: f.Ptr("My app description"),
			},
			func(ctx context.Context, req *http.Request) error { return nil },
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, createResp.StatusCode())
		require.NotNil(t, createResp.JSON201)

		respBrick, err := httpClient.UpsertAppBrickInstanceWithResponse(
			t.Context(),
			*createResp.JSON201.Id,
			"arduino:streamlit_ui",
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
				Icon:        f.Ptr("💻"),
				Name:        "test-app-2",
				Description: f.Ptr("My app description"),
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

func TestGetAppsStatusEvents(t *testing.T) {

	httpClient := GetHttpclient(t)
	appName := "example-app-for-status-events"

	t.Run("StreamAppEvents_Success", func(t *testing.T) {
		eventsResp, err := httpClient.GetAppsEvents(t.Context())
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, eventsResp.StatusCode)
		defer eventsResp.Body.Close()
		scanner := bufio.NewScanner(eventsResp.Body)
		go func() {
			createResp, err := httpClient.CreateAppWithResponse(
				t.Context(),
				&client.CreateAppParams{SkipSketch: f.Ptr(true)},
				client.CreateAppRequest{
					Icon:        f.Ptr("💻"),
					Name:        appName,
					Description: f.Ptr("My app description"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, createResp.StatusCode())
			appResponse, err := httpClient.StartAppWithResponse(t.Context(), *createResp.JSON201.Id)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, appResponse.StatusCode())
		}()
		var lastStatuses []string
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			eventData := strings.TrimPrefix(line, "data: ")
			if strings.Contains(eventData, `"name":"`+appName+`"`) {
				var event map[string]interface{}
				err := json.Unmarshal([]byte(eventData), &event)
				require.NoError(t, err)
				status, ok := event["status"].(string)
				require.True(t, ok, "status field missing or not string")
				lastStatuses = append(lastStatuses, status)
				if len(lastStatuses) > 3 {
					lastStatuses = lastStatuses[1:]
				}
				if len(lastStatuses) == 3 &&
					lastStatuses[0] == "stopped" &&
					lastStatuses[1] == "running" &&
					lastStatuses[2] == "stopped" {
					fmt.Println("Desired sequence received, terminating test")
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(fmt.Errorf("error reading event stream: %w", err))
		}

	})
}

func TestAppList(t *testing.T) {
	httpClient := GetHttpclient(t)
	t.Run("AppListEmpty_success", func(t *testing.T) {
		resp, err := httpClient.GetAppsWithResponse(t.Context(), &client.GetAppsParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Empty(t, resp.JSON200.Apps, "The apps list should be empty")
	})

	t.Run("AppListShouldContainsAllTheelements_success", func(t *testing.T) {
		expectedAppNumber := 5
		for i := 0; i < expectedAppNumber; i++ {
			r, err := httpClient.CreateApp(t.Context(), &client.CreateAppParams{
				SkipSketch: f.Ptr(false),
			}, client.CreateAppRequest{
				Icon:        f.Ptr("🌎"),
				Name:        "HelloWorld-" + strconv.Itoa(i),
				Description: f.Ptr("My HelloWorld description")})
			require.NoError(t, err)
			defer r.Body.Close()
		}
		resp, err := httpClient.GetAppsWithResponse(t.Context(), &client.GetAppsParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, len(*resp.JSON200.Apps), expectedAppNumber, "The apps list should contain "+strconv.Itoa(expectedAppNumber)+" elements")
	})

	t.Run("AppListDefault_success", func(t *testing.T) {
		r, err := httpClient.CreateApp(t.Context(), &client.CreateAppParams{
			SkipSketch: f.Ptr(false),
		}, client.CreateAppRequest{
			Icon:        f.Ptr("🌎"),
			Name:        "HelloWorld-default",
			Description: f.Ptr("My HelloWorld description")})
		require.NoError(t, err)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, r.StatusCode)
		var createdApp client.CreateAppResponse
		err = json.Unmarshal(body, &createdApp)
		require.NoError(t, err)
		require.NotNil(t, createdApp.Id)
		defaultAppId := *createdApp.Id

		editResp, err := httpClient.EditAppWithResponse(
			t.Context(),
			defaultAppId,
			client.EditRequest{
				Default: f.Ptr(true),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, editResp.StatusCode())

		resp, err := httpClient.GetAppsWithResponse(t.Context(), &client.GetAppsParams{Filter: f.Ptr("default")})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, 1, len(*resp.JSON200.Apps), "The apps list should contain 1 element")
		require.Equal(t, true, *(*resp.JSON200.Apps)[0].Default, "The app should be default")
		app := (*resp.JSON200.Apps)[0]
		require.Equal(t, "HelloWorld-default", *app.Name, "The app name should be 'HelloWorld-default'")
	})
}

func TestExportApp(t *testing.T) {
	httpClient := GetHttpclient(t)

	appName := "AppToExport"
	rootFolder := strings.ToLower(appName)
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon:        f.Ptr("📦"),
			Name:        appName,
			Description: f.Ptr("An app to test export functionality"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	validAppId := *createResp.JSON201.Id

	readZipFiles := func(t *testing.T, body io.Reader) []string {
		bodyBytes, err := io.ReadAll(body)
		require.NoError(t, err)

		zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
		require.NoError(t, err, "Response body is not a valid zip archive")

		var files []string
		for _, f := range zipReader.File {
			files = append(files, f.Name)
		}
		return files
	}

	t.Run("ExportDefault_Success", func(t *testing.T) {
		exportResp, err := httpClient.ExportApp(
			t.Context(),
			validAppId,
			&client.ExportAppParams{},
		)
		require.NoError(t, err)
		defer exportResp.Body.Close()

		require.Equal(t, http.StatusOK, exportResp.StatusCode)
		require.Equal(t, "application/zip", exportResp.Header.Get("Content-Type"))
		require.Contains(t, exportResp.Header.Get("Content-Disposition"), "attachment; filename=")

		files := readZipFiles(t, exportResp.Body)
		assert.Contains(t, files, rootFolder+"/app.yaml")
		assert.Contains(t, files, rootFolder+"/python/main.py")
		assert.NotContains(t, files, rootFolder+"/.cache")
	})

	t.Run("ExportWithIncludeData_Success", func(t *testing.T) {
		exportResp, err := httpClient.ExportApp(
			t.Context(),
			validAppId,
			&client.ExportAppParams{
				IncludeData: f.Ptr(true),
			},
		)
		require.NoError(t, err)
		defer exportResp.Body.Close()

		require.Equal(t, http.StatusOK, exportResp.StatusCode)
		require.Equal(t, "application/zip", exportResp.Header.Get("Content-Type"))
		files := readZipFiles(t, exportResp.Body)
		assert.Contains(t, files, rootFolder+"/app.yaml")
	})

	t.Run("InvalidAppId_Fail", func(t *testing.T) {
		malformedId := "user:test-plain-text"

		exportResp, err := httpClient.ExportApp(
			t.Context(),
			malformedId,
			&client.ExportAppParams{},
		)
		require.NoError(t, err)
		defer exportResp.Body.Close()

		require.Equal(t, http.StatusPreconditionFailed, exportResp.StatusCode)

		var actualResponseBody models.ErrorResponse
		body, err := io.ReadAll(exportResp.Body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Equal(t, "invalid id: illegal base64 data at input byte 4", actualResponseBody.Details)
	})

	t.Run("NonExistentAppId_Fail", func(t *testing.T) {
		nonExistentId := "dXNlcjpub24tZXhpc3RlbnQtYXBw"

		exportResp, err := httpClient.ExportApp(
			t.Context(),
			nonExistentId,
			&client.ExportAppParams{},
		)
		require.NoError(t, err)
		defer exportResp.Body.Close()

		require.Equal(t, http.StatusNotFound, exportResp.StatusCode)

		var actualResponseBody models.ErrorResponse
		body, err := io.ReadAll(exportResp.Body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &actualResponseBody)
		require.NoError(t, err)
		require.Contains(t, actualResponseBody.Details, "app path is not valid")
	})
}

func TestImportApp(t *testing.T) {
	httpClient := GetHttpclient(t)

	createZipBytes := func(t *testing.T, files map[string]string) []byte {
		t.Helper()
		buf := new(bytes.Buffer)
		zipWriter := zip.NewWriter(buf)

		for name, content := range files {
			f, err := zipWriter.Create(name)
			require.NoError(t, err)
			_, err = f.Write([]byte(content))
			require.NoError(t, err)
		}
		require.NoError(t, zipWriter.Close())
		return buf.Bytes()
	}

	createMultipartBody := func(t *testing.T, zipData []byte) (*bytes.Buffer, string) {
		t.Helper()
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file", "test-app.zip")
		require.NoError(t, err)
		_, err = part.Write(zipData)
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		return body, writer.FormDataContentType()
	}
	t.Run("Import_ValidNestedApp_Success", func(t *testing.T) {
		appFolderName := "my-nested-app"
		rootPrefix := "random-folder-root"

		zipData := createZipBytes(t, map[string]string{
			rootPrefix + "/app.yaml":       fmt.Sprintf("name: %s\ndescription: nested app", appFolderName),
			rootPrefix + "/python/main.py": "print('Hello nested world')",
		})
		bodyBuf, contentType := createMultipartBody(t, zipData)

		importResp, err := httpClient.ImportAppWithBody(
			t.Context(),
			&client.ImportAppParams{},
			contentType,
			bodyBuf,
		)
		require.NoError(t, err)

		require.Equal(t, http.StatusCreated, importResp.StatusCode)
		require.NotNil(t, importResp.Body)
		defer importResp.Body.Close()

		var importRespBody handlers.AppImportResponse
		body, err := io.ReadAll(importResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &importRespBody)
		require.NoError(t, err)
		require.NotNil(t, importRespBody.ID)

		getResp, err := httpClient.GetAppDetailsWithResponse(t.Context(), importRespBody.ID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, getResp.StatusCode())
		require.Equal(t, appFolderName, getResp.JSON200.Name)
	})

	t.Run("Import_ValidApp_Success", func(t *testing.T) {
		appFolderName := "test-app"

		zipData := createZipBytes(t, map[string]string{
			"app.yaml":       "name: my app \ndescription: my app",
			"python/main.py": "print('Hello imported world')",
		})
		bodyBuf, contentType := createMultipartBody(t, zipData)

		importResp, err := httpClient.ImportAppWithBody(
			t.Context(),
			&client.ImportAppParams{},
			contentType,
			bodyBuf,
		)
		require.NoError(t, err)

		require.Equal(t, http.StatusCreated, importResp.StatusCode)
		require.NotNil(t, importResp.Body)
		defer importResp.Body.Close()

		var importRespBody handlers.AppImportResponse
		body, err := io.ReadAll(importResp.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &importRespBody)
		require.NoError(t, err)
		require.NotNil(t, importRespBody.ID)

		importedAppId := importRespBody.ID
		getResp, err := httpClient.GetAppDetailsWithResponse(t.Context(), importedAppId)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, getResp.StatusCode())
		require.Equal(t, "my app", getResp.JSON200.Name)
		expectedID := base64.RawStdEncoding.EncodeToString([]byte("user:" + appFolderName))
		require.Equal(t, expectedID, getResp.JSON200.Id)
	})
	t.Run("Import_DeeplyNestedApp_Fail", func(t *testing.T) {
		appFolderName := "my-nested-app"
		rootPrefix := "random-folder-root/nested-levels"

		zipData := createZipBytes(t, map[string]string{
			rootPrefix + "/app.yaml":       fmt.Sprintf("name: %s\ndescription: nested app", appFolderName),
			rootPrefix + "/python/main.py": "print('Hello nested world')",
		})
		bodyBuf, contentType := createMultipartBody(t, zipData)

		importResp, err := httpClient.ImportAppWithBody(
			t.Context(),
			&client.ImportAppParams{},
			contentType,
			bodyBuf,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, importResp.StatusCode)

		require.NotNil(t, importResp.Body)
		defer importResp.Body.Close()

		bodyBytes, err := io.ReadAll(importResp.Body)
		require.NoError(t, err)
		var errorResponse models.ErrorResponse
		err = json.Unmarshal(bodyBytes, &errorResponse)
		require.NoError(t, err)
		expectedMsg := "bad request: invalid archive structure: missing or misplaced app.yaml. Supported paths: archive.zip/app.yaml or archive.zip/<root_dir>/app.yaml"
		require.Equal(t, expectedMsg, errorResponse.Details)

	})
	t.Run("Import_MissingAppYaml_Fail", func(t *testing.T) {
		zipData := createZipBytes(t, map[string]string{
			"python/main.py": "print('No app.yaml here')",
		})

		bodyBuf, contentType := createMultipartBody(t, zipData)

		importResp, err := httpClient.ImportAppWithBody(
			t.Context(),
			&client.ImportAppParams{},
			contentType,
			bodyBuf,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, importResp.StatusCode)

		require.NotNil(t, importResp.Body)
		defer importResp.Body.Close()

		bodyBytes, err := io.ReadAll(importResp.Body)
		require.NoError(t, err)
		var errorResponse models.ErrorResponse
		err = json.Unmarshal(bodyBytes, &errorResponse)
		require.NoError(t, err)
		expectedMsg := "bad request: invalid archive structure: missing or misplaced app.yaml. Supported paths: archive.zip/app.yaml or archive.zip/<root_dir>/app.yaml"
		require.Equal(t, expectedMsg, errorResponse.Details)

	})

	t.Run("Import_InvalidZip_Fail", func(t *testing.T) {
		fakeZipData := []byte("not valid zip content")

		bodyBuf, contentType := createMultipartBody(t, fakeZipData)

		importResp, err := httpClient.ImportAppWithBody(
			t.Context(),
			&client.ImportAppParams{},
			contentType,
			bodyBuf,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, importResp.StatusCode)

		require.NotNil(t, importResp.Body)
		defer importResp.Body.Close()

		bodyBytes, err := io.ReadAll(importResp.Body)
		require.NoError(t, err)
		var errorResponse models.ErrorResponse
		err = json.Unmarshal(bodyBytes, &errorResponse)
		require.NoError(t, err)
		expectedMsg := "unable to open zip archive: zip: not a valid zip file"
		require.Equal(t, expectedMsg, errorResponse.Details)
	})
	t.Run("Import_EmptyAppName_Fail", func(t *testing.T) {
		zipData := createZipBytes(t, map[string]string{
			"app.yaml":       "name: '   '",
			"python/main.py": "print('ok')",
		})

		bodyBuf, contentType := createMultipartBody(t, zipData)

		importResp, err := httpClient.ImportAppWithBody(
			t.Context(),
			&client.ImportAppParams{},
			contentType,
			bodyBuf,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, importResp.StatusCode)

		require.NotNil(t, importResp.Body)
		defer importResp.Body.Close()

		bodyBytes, err := io.ReadAll(importResp.Body)
		require.NoError(t, err)
		var errorResponse models.ErrorResponse
		err = json.Unmarshal(bodyBytes, &errorResponse)
		require.NoError(t, err)

		expectedMsg := "bad request: app name is missing"
		require.Equal(t, expectedMsg, errorResponse.Details)
	})
	t.Run("Import_Conflict_Fail", func(t *testing.T) {
		appName := "conflict-app"
		zipData := createZipBytes(t, map[string]string{
			"app.yaml":       fmt.Sprintf("name: %s", appName),
			"python/main.py": "pass",
		})

		bodyBuf1, contentType1 := createMultipartBody(t, zipData)
		resp1, err := httpClient.ImportAppWithBody(
			t.Context(),
			&client.ImportAppParams{},
			contentType1,
			bodyBuf1,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp1.StatusCode)
		resp1.Body.Close()

		bodyBuf2, contentType2 := createMultipartBody(t, zipData)
		resp2, err := httpClient.ImportAppWithBody(
			t.Context(),
			&client.ImportAppParams{},
			contentType2,
			bodyBuf2,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, resp2.StatusCode)

		require.NotNil(t, resp2.Body)
		defer resp2.Body.Close()

		bodyBytes, err := io.ReadAll(resp2.Body)
		require.NoError(t, err)
		var errorResponse models.ErrorResponse
		err = json.Unmarshal(bodyBytes, &errorResponse)
		require.NoError(t, err)

		expectedMsg := "app already exists"
		require.Equal(t, expectedMsg, errorResponse.Details)
	})
}

func TestSketchAppLibrariesCommands(t *testing.T) {
	httpClient := GetHttpclient(t)

	// Create a new App
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(false)},
		client.CreateAppRequest{
			Icon:        f.Ptr("📚"),
			Name:        "test-app-libraries",
			Description: f.Ptr("Test app for library operations"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)
	appID := *createResp.JSON201.Id

	// Install "Arduino_RouterBridge" library with dependencies
	addResp, err := httpClient.AppSketchAddLibraryWithResponse(
		t.Context(),
		appID,
		"Arduino_RouterBridge",
		&client.AppSketchAddLibraryParams{AddDeps: f.Ptr(true)},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, addResp.StatusCode())
	require.NotNil(t, addResp.JSON200)
	require.NotNil(t, addResp.JSON200.Libraries)
	require.NotEmpty(t, *addResp.JSON200.Libraries, "Added libraries list should not be empty")

	// List libraries and verify "Arduino_RouterBridge" is in the list with its dependencies
	listResp, err := httpClient.AppSketchListLibrariesWithResponse(
		t.Context(),
		appID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listResp.StatusCode())
	require.NotNil(t, listResp.JSON200)
	require.NotNil(t, listResp.JSON200.Libraries)
	require.NotEmpty(t, *listResp.JSON200.Libraries, "Libraries list should not be empty")

	// Verify Arduino_RouterBridge is in the list
	libraries := *listResp.JSON200.Libraries
	dependencies := *listResp.JSON200.Dependencies
	foundRouterBridge := false
	for _, lib := range libraries {
		if strings.Contains(lib, "Arduino_RouterBridge") {
			foundRouterBridge = true
		}
	}
	require.True(t, foundRouterBridge, "Arduino_RouterBridge should be in the libraries list")
	require.Greater(t, len(dependencies), 1, "Should have at least one dependency")

	// Remove library with dependencies
	removeResp, err := httpClient.AppSketchRemoveLibraryWithResponse(
		t.Context(),
		appID,
		"Arduino_RouterBridge",
		&client.AppSketchRemoveLibraryParams{RemoveDeps: f.Ptr(true)},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, removeResp.StatusCode())
	require.NotNil(t, removeResp.JSON200)
	require.NotNil(t, removeResp.JSON200.Libraries)
	require.NotEmpty(t, *removeResp.JSON200.Libraries, "Removed libraries list should not be empty")

	// List libraries again and verify the library is removed
	finalListResp, err := httpClient.AppSketchListLibrariesWithResponse(
		t.Context(),
		appID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, finalListResp.StatusCode())
	require.NotNil(t, finalListResp.JSON200)

	// Verify Arduino_RouterBridge is no longer in the list
	if finalListResp.JSON200.Libraries != nil {
		finalLibraries := *finalListResp.JSON200.Libraries
		for _, lib := range finalLibraries {
			require.NotContains(t, lib, "Arduino_RouterBridge", "Arduino_RouterBridge should be removed from the list")
		}
	}
}

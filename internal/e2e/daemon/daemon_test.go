package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tmaxmax/go-sse"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestCreateApp(t *testing.T) {
	// setup
	cli := e2e.CreateEnvForDaemon(t)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClient(cli.DaemonAddr)
	require.NoError(t, err)
	bricks := []string{"brick_1"}
	defaultRequestBody := client.CreateAppRequest{
		Icon:   f.Ptr("🌎"),
		Name:   "HelloWorld",
		Bricks: &bricks,
	}
	// tests
	testCases := []struct {
		name               string
		parameters         client.CreateAppParams
		body               client.CreateAppRequest
		expectedStatusCode int
		customAssertFunc   func(t *testing.T, body []byte)
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
			body:               defaultRequestBody,
			expectedStatusCode: http.StatusConflict,
		},
		{
			name: "should return 201 Created on successful creation",
			parameters: client.CreateAppParams{
				SkipPython: f.Ptr(true),
				SkipSketch: f.Ptr(false),
			},
			body: client.CreateAppRequest{
				Icon:   f.Ptr("🌎"),
				Name:   "HelloWorld_2",
				Bricks: &bricks,
			},
			expectedStatusCode: http.StatusCreated,
		},
		{
			name: "should return 201 Created on successful creation",
			parameters: client.CreateAppParams{
				SkipPython: f.Ptr(false),
				SkipSketch: f.Ptr(true),
			},
			body: client.CreateAppRequest{
				Icon:   f.Ptr("🌎"),
				Name:   "HelloWorld_3",
				Bricks: &bricks,
			},
			expectedStatusCode: http.StatusCreated,
		},
		{
			name: "should return 400 Bad Request when creating an app with both filters set to true",
			parameters: client.CreateAppParams{
				SkipPython: f.Ptr(true),
				SkipSketch: f.Ptr(true),
			},
			body:               defaultRequestBody,
			expectedStatusCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := httpClient.CreateApp(t.Context(), &tc.parameters, tc.body)
			require.NoError(t, err)
			defer r.Body.Close()
			require.Equal(t, tc.expectedStatusCode, r.StatusCode)

			if tc.customAssertFunc != nil {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				tc.customAssertFunc(t, body)
			}
		})
	}
}

func TestAIModelList(t *testing.T) {
	// setup
	cli := e2e.CreateEnvForDaemon(t)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)

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
	cli := e2e.CreateEnvForDaemon(t)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)

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

func TestBricksList(t *testing.T) {
	// setup
	cli := e2e.CreateEnvForDaemon(t)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)

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

func TestEditApp(t *testing.T) {
	// setup
	cli := e2e.CreateEnvForDaemon(t)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)

	appName := "test-app"
	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: f.Ptr(true)},
		client.CreateAppRequest{
			Icon: f.Ptr("💻"),
			Name: appName,
		},
		func(ctx context.Context, req *http.Request) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	renamedApp := appName + "-renamed"
	editResp, err := httpClient.EditAppWithResponse(
		t.Context(),
		*createResp.JSON201.Id,
		client.EditRequest{
			Description: f.Ptr("new-description"),
			Icon:        f.Ptr("🌟"),
			Name:        f.Ptr(renamedApp),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, editResp.StatusCode())

	// Verify the app was renamed
	appList, err := httpClient.GetAppsWithResponse(t.Context(), &client.GetAppsParams{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, appList.StatusCode())
	require.NotEmpty(t, appList.JSON200.Apps)
	require.Len(t, *appList.JSON200.Apps, 1)

	app := (*appList.JSON200.Apps)[0]
	require.Equal(t, renamedApp, *app.Name)
	require.Equal(t, "new-description", *app.Description)
	require.Equal(t, "🌟", *app.Icon)
}

func TestSystemResources(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("System resources test is only applicable for Linux")
	}

	// setup
	cli := e2e.CreateEnvForDaemon(t)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)

	//nolint:bodyclose
	systemResources, err := httpClient.GetSystemResources(t.Context())
	require.NoError(t, err)

	reqCtx, cancelCtx := context.WithTimeout(t.Context(), 1*time.Minute)
	conn := sse.DefaultClient.NewConnection(systemResources.Request.WithContext(reqCtx))

	var (
		cpuResp  orchestrator.SystemCPUResource
		memResp  orchestrator.SystemMemoryResource
		diskResp orchestrator.SystemDiskResource
	)

	conn.SubscribeToAll(func(event sse.Event) {
		switch event.Type {
		case "cpu":
			require.NoError(t, json.Unmarshal([]byte(event.Data), &cpuResp))
		case "mem":
			require.NoError(t, json.Unmarshal([]byte(event.Data), &memResp))
		case "disk":
			require.NoError(t, json.Unmarshal([]byte(event.Data), &diskResp))
		}
		if cpuResp != (orchestrator.SystemCPUResource{}) &&
			memResp != (orchestrator.SystemMemoryResource{}) &&
			diskResp != (orchestrator.SystemDiskResource{}) {
			cancelCtx() // Stop the connection once we have all resources
		}
	})

	err = conn.Connect()
	if !errors.Is(err, context.Canceled) {
		require.NoError(t, err)
	}
	require.NotEmpty(t, cpuResp.UsedPercent)
	require.NotEmpty(t, memResp.Used)
	require.NotEmpty(t, memResp.Total)
	require.NotEmpty(t, diskResp.Path)
	require.NotEmpty(t, diskResp.Used)
	require.NotEmpty(t, diskResp.Total)
}

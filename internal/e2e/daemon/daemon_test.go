package daemon

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
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

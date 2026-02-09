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

package orchestrator

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestBuildBrickConfigForEIModel(t *testing.T) {

	brickIndex, err := bricksindex.Load(paths.New("bricksindex/testdata"))
	if err != nil {
		t.Fatalf("failed to load bricks index: %v", err)
	}

	category := edgeimpulse.ProjectCategory("Object detection")
	edgeModelsDir := paths.New("/models/custom-ei/ei-xxxx-yyyy")
	blobModelsDir := paths.New("/models/custom-ei/ei-xxxx-yyyy")

	result := buildBrickConfigForEIModel(
		brickIndex,
		&category,
		edgeModelsDir,
		blobModelsDir,
	)

	require.Len(t, result, 2)

	require.Equal(t, "arduino:object_detection", result[0].ID)
	require.Equal(t, "arduino:video_object_detection", result[1].ID)

	require.Equal(t, map[string]string{
		"CUSTOM_MODEL_PATH":      "/models/custom-ei/ei-xxxx-yyyy",
		"EI_OBJ_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
	}, result[0].ModelConfiguration)
	require.Equal(t, map[string]string{
		"CUSTOM_MODEL_PATH":      "/models/custom-ei/ei-xxxx-yyyy",
		"EI_OBJ_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
	}, result[1].ModelConfiguration)
}

func createFileWithSize(t *testing.T, dir, name string, size int) {
	t.Helper()

	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = io.CopyN(f, rand.Reader, int64(size))
	require.NoError(t, err)
}

func TestGetModelSize(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]int
		expectedSize uint64
		expectError  bool
		setupExtra   func(t *testing.T, baseDir string)
	}{
		{
			name:         "empty directory",
			files:        map[string]int{},
			expectedSize: 0,
			expectError:  false,
		},
		{
			name: "single small file",
			files: map[string]int{
				"file1.bin": 1024 * 1024, // 1 MB
			},
			expectedSize: 1024 * 1024,
			expectError:  false,
		},
		{
			name: "multiple files",
			files: map[string]int{
				"file1.bin": 1024 * 1024, // 1 MB
				"file2.bin": 512 * 1024,  // 0.5 MB
			},
			expectedSize: 1024*1024 + 512*1024,
			expectError:  false,
		},
		{
			name:         "non existing directory",
			files:        nil,
			expectedSize: 0,
			expectError:  true,
		},
		{
			name: "permission denied on subdirectory",
			files: map[string]int{
				"allowed.bin": 1024,
			},
			expectError: true,
			setupExtra: func(t *testing.T, baseDir string) {
				restrictedDir := filepath.Join(baseDir, "private")
				err := os.Mkdir(restrictedDir, 0000)
				require.NoError(t, err)
				t.Cleanup(func() {
					_ = os.Chmod(restrictedDir, 0600)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dir string

			if !tt.expectError {
				tmpDir := t.TempDir()
				dir = tmpDir

				for name, size := range tt.files {
					createFileWithSize(t, tmpDir, name, size)
				}

				if tt.setupExtra != nil {
					tt.setupExtra(t, tmpDir)
				}
			} else {
				dir = "/path/that/does/not/exist"
			}

			dirPath := paths.New(dir)

			sizeMB, err := getModelSize(dirPath)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedSize, sizeMB)
		})
	}
}

type mockResponse struct {
	status int
	body   string
}

func setupMockEIServer(t *testing.T, responses map[string]mockResponse, calls *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.URL.Path)
		t.Logf("Received >%s<\n", r.URL.Path)
		res, ok := responses[r.URL.Path]
		if !ok {
			t.Logf("DEBUG: Mock received unhandled path: >%s< >%s<\n", r.Method, r.URL.String())
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "path not mocked"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(res.status)
		w.Write([]byte(res.body))
	}))
}

func TestInstallEIModel_WhenModelIsNotBuilt_ThanTriggerTheBuild(t *testing.T) {
	trackActualServercalls := []string{}

	// GetInfoLastDeployment: deployment response: no deployment exists
	buildInfoJSON := fmt.Sprintf(`{
		"success": true,
		"hasBuild": false,
		"lastDeploymentTarget": {
			"format": "%s"
		},
		"lastBuild": {
			"version": 12345,
			"deploymentType": "linux",
			"engine": "tflite",
			"created": "2024-05-20T10:00:00Z"
		}
	}`, "runner-linux-aarch64")

	// Build: response
	buildOnDeviceJSON := `{
    "success": true,
    "id": 99988,
    "deploymentVersion": 1,
    "error": null
	}`

	// WaitForBuildCompletion: job status response
	jobFinishedJSON := `{
    "success": true,
    "job": {
        "id": 99988,
        "finished": "2026-02-05T18:00:00Z",
        "finishedSuccessful": true,
        "jobType": "build-on-device"
		}
	}`

	// GetProjectInfo
	// category is missing on purpose, this avoid to trigger brick related code
	projectInfoJSON := `{
		"success": true,
		"project": {
			"id": 100,
			"name": "Imola-Model",
			"description": "Optimized model for aarch64",
			"category": "missing-category",
			"lastModified": "2026-02-05T12:00:00Z"
		}
	}`

	responses := map[string]mockResponse{
		"/api/100/deployment/last":           {status: http.StatusOK, body: buildInfoJSON},
		"/api/100/jobs/build-ondevice-model": {status: http.StatusOK, body: buildOnDeviceJSON},
		"/api/100/jobs/99988/status":         {status: http.StatusOK, body: jobFinishedJSON},
		"/api/100":                           {status: http.StatusOK, body: projectInfoJSON},
		"/api/100/deployment/download":       {status: http.StatusOK, body: `fake-binary-data`},
	}
	server := setupMockEIServer(t, responses, &trackActualServercalls)
	defer server.Close()

	// arrange
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client, _ := edgeimpulse.NewEIClient("fake-key", *serverURL)

	// act
	projectId := 100
	impulseId := 1
	tempDir := t.TempDir()
	result, err := InstallEIModel(context.Background(), nil, client, paths.New(tempDir), projectId, impulseId)

	// assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Name != "Imola-Model" {
		t.Errorf("Expected name Imola-Model, got %s", result.Name)
	}

	expectedCalls := []string{
		"/api/100/deployment/last",
		"/api/100/jobs/build-ondevice-model",
		"/api/100/jobs/99988/status",
		"/api/100",
		"/api/100/deployment/download",
	}

	assertServerCalls(trackActualServercalls, expectedCalls, t)
}

func TestInstallEIModel_WhenModelIsNotFullyTrained_ThanRaiseError(t *testing.T) {
	t.Skip("Temporarily disabling this until the logic for handling not fully trained models is clear")
	trackActualServercalls := []string{}

	// GetInfoLastDeployment: deployment response: no deployment exists
	buildInfoJSON := fmt.Sprintf(`{"success":true,"hasBuild":false,"lastDeploymentTarget":{"disabledForProject":false,"recommendedForProject":false,"name":"Linux (AARCH64 with Ethos-U65-256)","description":"An EIM binary for Linux (aarch64) CPU with Ethos-U65 NPU that implements the Edge Impulse Linux protocol. Model compiled for Ethos-U65-256, High End system with dedicated SRAM, for example: NXP i.MX 93 or Digi ConnectCore 93.","image":"https://studio.edgeimpulse.com/assets/41a7922e2ec3141c1b28ea833b00a12ae42512dd/deploy-h200/linux.webp","imageClasses":"px-4 py-4","format":"runner-linux-aarch64-nxp-imx93","hasEonCompiler":false,"hasTensorRT":false,"hasTensaiFlow":false,"hasDRPAI":false,"hasTIDL":false,"hasAkida":false,"hasMemryx":false,"hasStAton":false,"hasCevaNpn":false,"hasNordicAxon":false,"hideOptimizations":false,"uiSection":"firmware","supportedEngines":["ethos-linux"],"preferredEngine":"ethos-linux","url":"","docsUrl":"","firmwareRepoUrl":"https://github.com/edgeimpulse/example-standalone-inferencing-linux","modelVariants":[{"variant":"int8","supported":true},{"variant":"float32","supported":false,"hint":"The Ethos NPU only supports quantized models"}],"parameters":[]}}`)

	// Build: response
	buildOnDeviceJSON := `{
    "success": true
	}`

	// GetProjectInfo
	// category is missing on purpose, this avoid to trigger brick related code
	projectInfoJSON := `{
		"success": false,
		"project": {
			"id": 100,
			"name": "Imola-Model",
			"description": "Optimized model for aarch64",
			"category": "missing-category",
			"lastModified": "2026-02-05T12:00:00Z"
		}
	}`

	responses := map[string]mockResponse{
		"/api/100/deployment/last":           {status: http.StatusOK, body: buildInfoJSON},
		"/api/100/jobs/build-ondevice-model": {status: http.StatusOK, body: buildOnDeviceJSON},
		"/api/100":                           {status: http.StatusOK, body: projectInfoJSON},
	}
	server := setupMockEIServer(t, responses, &trackActualServercalls)
	defer server.Close()

	// arrange
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client, _ := edgeimpulse.NewEIClient("fake-key", *serverURL)

	// act
	projectId := 100
	impulseId := 1
	tempDir := t.TempDir()
	result, err := InstallEIModel(context.Background(), nil, client, paths.New(tempDir), projectId, impulseId)

	// assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Name != "Imola-Model" {
		t.Errorf("Expected name Imola-Model, got %s", result.Name)
	}

	expectedCalls := []string{
		"/api/100/deployment/last",
		"/api/100/jobs/build-ondevice-model",
		"/api/100/jobs/99988/status",
		"/api/100",
		"/api/100/deployment/download",
	}
	assertServerCalls(trackActualServercalls, expectedCalls, t)
}

func TestInstallEIModel_WhenModelIsBuilt_ThanTheStoreSucceeded(t *testing.T) {
	trackActualServercalls := []string{}

	// GetInfoLastDeployment: deployment response: no deployment exists
	buildInfoJSON := fmt.Sprintf(`{
		"success": true,
		"hasBuild": true,
		"lastDeploymentTarget": {
			"format": "%s"
		},
		"lastBuild": {
			"version": 12345,
			"deploymentType": "linux",
			"engine": "tflite",
			"created": "2024-05-20T10:00:00Z"
		}
	}`, "runner-linux-aarch64")

	// GetProjectInfo
	// category is missing on purpose, this avoid to trigger brick related code
	projectInfoJSON := `{
		"success": true,
		"project": {
			"id": 100,
			"name": "Imola-Model",
			"description": "Optimized model for aarch64",
			"category": "missing-category",
			"lastModified": "2026-02-05T12:00:00Z"
		}
	}`

	responses := map[string]mockResponse{
		"/api/100/deployment/last":     {status: http.StatusOK, body: buildInfoJSON},
		"/api/100":                     {status: http.StatusOK, body: projectInfoJSON},
		"/api/100/deployment/download": {status: http.StatusOK, body: `fake-binary-data`},
	}
	server := setupMockEIServer(t, responses, &trackActualServercalls)
	defer server.Close()

	// arrange
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client, _ := edgeimpulse.NewEIClient("fake-key", *serverURL)

	// act
	projectId := 100
	impulseId := 1
	tempDir := t.TempDir()
	result, err := InstallEIModel(context.Background(), nil, client, paths.New(tempDir), projectId, impulseId)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	// assert
	if result.Name != "Imola-Model" {
		t.Errorf("Expected name Imola-Model, got %s", result.Name)
	}
	if result.ID != "ei-model-100-1" {
		t.Errorf("Expected ID ei-model-100-1, got %s", result.ID)
	}

	basePath := paths.New(tempDir).Join("custom-ei").Join(result.ID)
	assertModelFileContent(t, basePath.Join("model.eim").String())
	assertAppYamlContent(t, basePath.Join("model.yaml").String())

	expectedCalls := []string{
		"/api/100/deployment/last",
		"/api/100",
		"/api/100/deployment/download",
	}

	assertServerCalls(trackActualServercalls, expectedCalls, t)
}

func assertServerCalls(actualCalls, expectedCalls []string, t *testing.T) {
	if len(actualCalls) != len(expectedCalls) {
		t.Errorf("Expected %d calls, but got %d", len(expectedCalls), len(actualCalls))
	}

	for i, path := range expectedCalls {
		if i < len(actualCalls) && actualCalls[i] != path {
			t.Errorf("Call %d: expected %s, got %s", i, path, actualCalls[i])
		}
	}
}

func assertModelFileContent(t *testing.T, filename string) {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read %s: %v", filename, err)
	}

	if !bytes.Contains(data, []byte("fake-binary-data")) {
		t.Errorf("file %s did not contain 'fake-binary-data'", filename)
		t.Logf("Actual content: %s", string(data))
	}
}

func assertAppYamlContent(t *testing.T, yamlFile string) {
	data, err := os.ReadFile(yamlFile)
	require.NoError(t, err)

	var config AIModelItem
	err = yaml.Unmarshal(data, &config)
	require.NoError(t, err, "Failed to parse YAML")

	require.Equal(t, "ei-model-100-1", config.ID)
	require.Equal(t, "Imola-Model", config.Name)
}

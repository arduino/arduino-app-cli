// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package orchestrator

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestBuildBrickConfigForEIModel(t *testing.T) {
	var yamlContent = `
bricks:
- id: arduino:image_classification
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /opt/models/ei/
    description: path to the custom model directory
  - name: EI_CLASSIFICATION_MODEL
    default_value: /models/ootb/ei/mobilenet-v2-224px.eim
    description: path to the model file
- id: arduino:object_detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /opt/models/ei/
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file
- id: arduino:video_object_detection
  variables:
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: Path to the model file
    hidden: true
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: Path to the custom model directory
    hidden: true
- id: arduino:visual_anomaly_detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /opt/models/ei/
    description: path to the custom model directory
  - name: EI_V_ANOMALY_DETECTION_MODEL
    default_value: /models/ootb/ei/concrete-crack-anomaly-detection.eim
    description: path to the model file
- id: arduino:keyword_spotting
  variables:
  - name: EI_KEYWORD_SPOTTING_MODEL
    default_value: /models/ootb/ei/keyword-spotting-hey-arduino.eim
    description: Path to the model file
    hidden: true
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: Path to the custom model directory
    hidden: true
- id: arduino:audio_classification
  variables:
  - name: EI_AUDIO_CLASSIFICATION_MODEL
    default_value: /models/ootb/ei/glass-breaking.eim
    description: Path to the model file
    hidden: true
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: Path to the custom model directory
    hidden: true
- id: arduino:motion_detection
  variables:
  - name: EI_MOTION_DETECTION_MODEL
    default_value: /models/ootb/ei/updown-wave-motion-detection.eim
    description: Path to the model file
    hidden: true
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: Path to the custom model directory
    hidden: true
- id: arduino:vibration_anomaly_detection
  variables:
  - name: EI_VIBRATION_ANOMALY_DETECTION_MODEL
    default_value: /models/ootb/ei/fan-anomaly-detection.eim
    description: Path to the model file
    hidden: true
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: Path to the custom model directory
    hidden: true
- id: arduino:video_image_classification
  variables:
  - name: EI_V_CLASSIFICATION_MODEL
    default_value: /models/ootb/ei/mobilenet-v2-224px.eim
    description: Path to the model file
    hidden: true
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: Path to the custom model directory
    hidden: true
`
	assetDir := paths.TempDir()
	err := assetDir.Join("bricks-list.yaml").WriteFile([]byte(yamlContent))
	require.NoError(t, err)

	brickIndex, err := bricksindex.Load(platform.GetPlatform(nil), assetDir)
	if err != nil {
		t.Fatalf("failed to load bricks index: %v", err)
	}

	edgeModelsDir := paths.New("/models/custom-ei/ei-xxxx-yyyy")
	blobModelsDir := paths.New("/models/custom-ei/ei-xxxx-yyyy")

	tests := []struct {
		name           string
		category       edgeimpulse.ProjectCategory
		learnBlocks    []edgeimpulse.ImpulseLearnBlock
		expectedIDs    []string
		expectedConfig []map[string]string
	}{
		{
			name:        "object detection",
			category:    edgeimpulse.ProjectCategoryObjectDetection,
			learnBlocks: nil,
			expectedIDs: []string{
				"arduino:object_detection",
				"arduino:video_object_detection",
			},
			expectedConfig: []map[string]string{
				{
					"CUSTOM_MODEL_PATH":      "/models/custom-ei/ei-xxxx-yyyy",
					"EI_OBJ_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
				{
					"CUSTOM_MODEL_PATH":      "/models/custom-ei/ei-xxxx-yyyy",
					"EI_OBJ_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
			},
		},
		{
			name:     "Images with visual anomaly learning block",
			category: edgeimpulse.ProjectCategoryImages,
			learnBlocks: []edgeimpulse.ImpulseLearnBlock{
				{
					Type: edgeimpulse.KerasVisualAnomaly,
				},
			},
			expectedIDs: []string{
				"arduino:visual_anomaly_detection",
			},
			expectedConfig: []map[string]string{
				{
					"CUSTOM_MODEL_PATH":            "/models/custom-ei/ei-xxxx-yyyy",
					"EI_V_ANOMALY_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
			},
		},
		{
			name:        "Images without visual anomaly learning block",
			category:    edgeimpulse.ProjectCategoryImages,
			learnBlocks: nil,
			expectedIDs: []string{
				"arduino:image_classification",
				"arduino:video_image_classification"},

			expectedConfig: []map[string]string{
				{
					"CUSTOM_MODEL_PATH":       "/models/custom-ei/ei-xxxx-yyyy",
					"EI_CLASSIFICATION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
				{
					"CUSTOM_MODEL_PATH":         "/models/custom-ei/ei-xxxx-yyyy",
					"EI_V_CLASSIFICATION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
			},
		},
		{
			name:        "Keyword spotting",
			category:    edgeimpulse.ProjectCategoryKeywordSpotting,
			learnBlocks: nil,
			expectedIDs: []string{"arduino:audio_classification", "arduino:keyword_spotting"},
			expectedConfig: []map[string]string{
				{
					"CUSTOM_MODEL_PATH":             "/models/custom-ei/ei-xxxx-yyyy",
					"EI_AUDIO_CLASSIFICATION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
				{
					"CUSTOM_MODEL_PATH":         "/models/custom-ei/ei-xxxx-yyyy",
					"EI_KEYWORD_SPOTTING_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
			},
		},
		{
			name:        "Audio classification",
			category:    edgeimpulse.ProjectCategoryAudio,
			learnBlocks: nil,
			expectedIDs: []string{"arduino:audio_classification"},
			expectedConfig: []map[string]string{
				{
					"CUSTOM_MODEL_PATH":             "/models/custom-ei/ei-xxxx-yyyy",
					"EI_AUDIO_CLASSIFICATION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
			},
		},
		{
			name:        "Accelerometer",
			category:    edgeimpulse.ProjectCategoryAccelerometer,
			learnBlocks: nil,
			expectedIDs: []string{"arduino:motion_detection", "arduino:vibration_anomaly_detection"},
			expectedConfig: []map[string]string{
				{
					"CUSTOM_MODEL_PATH":         "/models/custom-ei/ei-xxxx-yyyy",
					"EI_MOTION_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
				{
					"CUSTOM_MODEL_PATH":                    "/models/custom-ei/ei-xxxx-yyyy",
					"EI_VIBRATION_ANOMALY_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildBrickConfigForEIModel(
				brickIndex,
				&tt.category,
				tt.learnBlocks,
				edgeModelsDir,
				blobModelsDir,
			)

			require.NoError(t, err)
			require.Len(t, result, len(tt.expectedIDs))

			for i, expectedID := range tt.expectedIDs {
				require.Equal(t, expectedID, result[i].ID)
				require.Equal(t, tt.expectedConfig[i], result[i].ModelConfiguration)
			}
		})
	}
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
			_, _ = w.Write([]byte(`{"error": "path not mocked"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(res.status)
		_, _ = w.Write([]byte(res.body))
	}))
}

func TestInstallEIModel_WhenModelIsNotBuilt_ThanTriggerTheBuild(t *testing.T) {
	trackActualServercalls := []string{}

	// GetProjectInfo
	projectInfoJSON := `{
		"success": true,
		"project": {
			"id": 100,
			"name": "Imola-Model",
			"description": "Optimized model for aarch64",
			"category": "missing-category",
			"lastModified": "2026-02-05T12:00:00Z"
		},
		"impulse": {
			"created": true,
			"configured": true,
			"complete": true
		}
  	}`

	// Build
	buildOnDeviceJSON := `{
    "success": true,
    "id": 99988,
    "deploymentVersion": 1,
    "error": null
   }`

	// WaitForBuildCompletion: job status response
	waitForbuildCompletionJSON := `{
		"success": true,
		"job": {
			"id": 99988,
			"finished": "2026-02-05T18:00:00Z",
			"finishedSuccessful": true,
			"jobType": "build-on-device"
		}
	}`

	// GetImpulseInfo
	impulseInfoJSON := `{
		"success": true,
		"impulse": {
			"id": 1,
			"name": "My Impulse",
			"created": true,
			"configured": true,
			"complete": true
		}
	}`

	responses := map[string]mockResponse{
		"/api/100":                               {status: http.StatusOK, body: projectInfoJSON},
		"/api/100/deployment/history":            {status: http.StatusOK, body: `{"success": true, "deployments": []}`},
		"/api/100/jobs/build-ondevice-model":     {status: http.StatusOK, body: buildOnDeviceJSON},
		"/api/100/jobs/99988/status":             {status: http.StatusOK, body: waitForbuildCompletionJSON},
		"/api/100/deployment/history/1/download": {status: http.StatusOK, body: `fake-binary-data`},
		"/api/100/impulse":                       {status: http.StatusOK, body: impulseInfoJSON},
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
	result, err := InstallEIModel(context.Background(), nil, &modelsindex.ModelsIndex{}, nil, client, paths.New(tempDir), projectId, impulseId)

	// assert
	require.NoError(t, err)
	require.Equal(t, "Imola-Model", result.Name)
	require.Equal(t, "edgeimpulse", result.Metadata["source"])

	// assert mock calls
	expectedCalls := []string{
		"/api/100",
		"/api/100/deployment/history",
		"/api/100/jobs/build-ondevice-model",
		"/api/100/jobs/99988/status",
		"/api/100/deployment/history/1/download",
		"/api/100/impulse",
	}
	assertServerCalls(trackActualServercalls, expectedCalls, t)
}

func TestInstallEIModel_WhenModelIsNotFullyTrained_ThanRaiseError(t *testing.T) {
	trackActualServercalls := []string{}

	// GetProjectInfo
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
		"/api/100": {status: http.StatusOK, body: projectInfoJSON},
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
	_, err = InstallEIModel(context.Background(), nil, &modelsindex.ModelsIndex{}, nil, client, paths.New(tempDir), projectId, impulseId)

	// assert
	require.Equal(t, "impulse not ready for deployment for project 100 impulse 1", err.Error())

	// assert mock calls
	expectedCalls := []string{
		"/api/100",
	}
	assertServerCalls(trackActualServercalls, expectedCalls, t)
}

func TestInstallEIModel_WhenModelIsBuilt_DoNotTriggerTheBuild_and_StoreSucceeded(t *testing.T) {
	trackActualServercalls := []string{}

	// GetProjectInfo
	projectInfoJSON := `{
		"success": true,
		"project": {
			"id": 100,
			"name": "Imola-Model",
			"description": "Optimized model for aarch64",
			"category": "missing-category",
			"lastModified": "2026-02-05T12:00:00Z"
		},
		"impulse": {
			"created": true,
			"configured": true,
			"complete": true
		}
  	}`

	// GetDeploymentHistory
	deploymentHistoryJson := `{
    "success": true,
    "totalDeploymentCount": 1,
    "deployments": [
        {
            "created": "2026-02-10T10:00:00Z",
            "deploymentFormat": "runner-linux-aarch64",
            "deploymentVersion": 5,
            "downloadUrl": "/api/v1/projects/100/deployment/download",
            "engine": "tflite",
            "modelType": "float32",
            "impulseHasChangedSinceDeployment": false,
            "impulseId": 1,
            "impulseIsDeleted": false,
            "impulseName": "Imola-Project"
        }
    ]
	}`

	// GetImpulseInfo
	impulseInfoJSON := `{
		"success": true,
		"impulse": {
			"id": 1,
			"name": "My Impulse",
			"created": true,
			"configured": true,
			"complete": true
		}
	}`

	responses := map[string]mockResponse{
		"/api/100":                               {status: http.StatusOK, body: projectInfoJSON},
		"/api/100/deployment/history":            {status: http.StatusOK, body: deploymentHistoryJson},
		"/api/100/deployment/history/5/download": {status: http.StatusOK, body: `fake-binary-data`},
		"/api/100/impulse":                       {status: http.StatusOK, body: impulseInfoJSON},
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
	result, err := InstallEIModel(context.Background(), nil, &modelsindex.ModelsIndex{}, nil, client, paths.New(tempDir), projectId, impulseId)

	// assert
	require.NoError(t, err)
	require.Equal(t, "Imola-Model", result.Name)
	require.Equal(t, "edgeimpulse", result.Metadata["source"])

	// assert write on disk
	basePath := paths.New(tempDir).Join("custom-ei").Join(result.ID)
	assertModelFileContent(t, basePath.Join("model.eim").String())
	assertAppYamlContent(t, basePath.Join("model.yaml").String())

	// assert mock calls
	expectedCalls := []string{
		"/api/100",
		"/api/100/deployment/history",
		"/api/100/deployment/history/5/download",
		"/api/100/impulse",
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

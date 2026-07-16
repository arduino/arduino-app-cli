// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import "testing"

func TestParseDockerImage(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedName    string
		expectedVersion string
	}{
		{
			name:            "Standard image with tag",
			input:           "nginx:latest",
			expectedName:    "nginx",
			expectedVersion: "latest",
		},
		{
			name:            "Image with digest (testing @ precedence)",
			input:           "my-service@sha256:8890123...123",
			expectedName:    "my-service",
			expectedVersion: "sha256:8890123...123",
		},
		{
			name:            "Image without version or tag",
			input:           "ubuntu",
			expectedName:    "ubuntu",
			expectedVersion: "",
		},
		{
			name:            "Registry path with tag",
			input:           "gcr.io/my-project/container-name:v1.2.3",
			expectedName:    "gcr.io/my-project/container-name",
			expectedVersion: "v1.2.3",
		},
		{
			name:            "Localhost with port and tag",
			input:           "localhost:5000/my-image:beta",
			expectedName:    "localhost:5000/my-image",
			expectedVersion: "beta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotVersion := parseDockerImage(tt.input)

			if gotName != tt.expectedName {
				t.Errorf("parseDockerImage(%q) Name = %q, want %q", tt.input, gotName, tt.expectedName)
			}

			if gotVersion != tt.expectedVersion {
				t.Errorf("parseDockerImage(%q) Version = %q, want %q", tt.input, gotVersion, tt.expectedVersion)
			}
		})
	}
}

func TestGetHighestVersion(t *testing.T) {
	tests := []struct {
		name           string
		targetImage    string
		existingImages []string
		expected       string
	}{
		{
			name:        "Selects highest semver",
			targetImage: "my-app",
			existingImages: []string{
				"my-app:1.0.0",
				"my-app:1.1.0",
				"my-app:1.0.1",
			},
			expected: "my-app:1.1.0",
		},
		{
			name:        "Skips invalid semver versions like latest",
			targetImage: "my-app",
			existingImages: []string{
				"my-app:latest",
				"my-app:1.2.0",
				"my-app:1.0.0",
			},
			expected: "my-app:1.2.0",
		},
		{
			name:        "Handles complex semver with prereleases",
			targetImage: "app",
			existingImages: []string{
				"app:1.0.0-rc.1",
				"app:1.0.0", // 1.0.0 > 1.0.0-rc.1
				"app:1.0.0-beta",
			},
			expected: "app:1.0.0",
		},
		{
			name:        "Returns empty if only 'latest' exists",
			targetImage: "my-app",
			existingImages: []string{
				"my-app:latest",
			},
			expected: "",
		},
		{
			name:        "Ignores images with different names",
			targetImage: "target-app",
			existingImages: []string{
				"other-app:5.0.0",
				"target-app:1.0.0",
			},
			expected: "target-app:1.0.0",
		},
		{
			name:           "Returns empty if list is empty",
			targetImage:    "my-app",
			existingImages: []string{},
			expected:       "",
		},
		{
			name:        "Returns empty if no name matches",
			targetImage: "my-app",
			existingImages: []string{
				"other:1.0.0",
				"foo:2.0.0",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetHighestVersion(tt.targetImage, tt.existingImages)
			if got != tt.expected {
				t.Errorf("GetHighestVersion() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSumUniqueLayers(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]dockerImageLayer
		expected int64
	}{
		{
			name:     "no images",
			input:    nil,
			expected: 0,
		},
		{
			name: "single image",
			input: [][]dockerImageLayer{
				{{Hash: "a", Size: 100}, {Hash: "b", Size: 50}},
			},
			expected: 150,
		},
		{
			name: "shared layer counted once",
			input: [][]dockerImageLayer{
				{{Hash: "base", Size: 200}, {Hash: "a", Size: 50}},
				{{Hash: "base", Size: 200}, {Hash: "b", Size: 30}},
			},
			expected: 280, // base(200) once + a(50) + b(30)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sumUniqueLayers(tt.input); got != tt.expected {
				t.Errorf("sumUniqueLayers() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestUpdateLayerProgress(t *testing.T) {
	layerProgress := map[string]int64{}

	// Two layers downloading in parallel: latest value per layer wins.
	if got := updateLayerProgress(layerProgress, "Downloading", "layer1", 100); got != 100 {
		t.Errorf("got %d, want 100", got)
	}
	if got := updateLayerProgress(layerProgress, "Downloading", "layer2", 50); got != 150 {
		t.Errorf("got %d, want 150", got)
	}
	if got := updateLayerProgress(layerProgress, "Downloading", "layer1", 300); got != 350 {
		t.Errorf("got %d, want 350 (layer1 updated, not added)", got)
	}

	// Extracting must not contribute (no double counting of the decompress phase).
	if got := updateLayerProgress(layerProgress, "Extracting", "layer1", 999); got != 350 {
		t.Errorf("got %d, want 350 (Extracting ignored)", got)
	}

	// Events without an ID must not contribute.
	if got := updateLayerProgress(layerProgress, "Downloading", "", 999); got != 350 {
		t.Errorf("got %d, want 350 (empty id ignored)", got)
	}
}

func TestImageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ghcr.io/arduino/app-bricks/python-apps-base:0.11.0rc6", "ghcr.io/arduino/app-bricks/python-apps-base"},
		{"influxdb:2.7-alpine", "influxdb"},
		{"ghcr.io/arduino/app-bricks/ei-models-runner@sha256:abc", "ghcr.io/arduino/app-bricks/ei-models-runner"},
		{"nginx", "nginx"},
	}
	for _, tt := range tests {
		if got := imageName(tt.input); got != tt.expected {
			t.Errorf("imageName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

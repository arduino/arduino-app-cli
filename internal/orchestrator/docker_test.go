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

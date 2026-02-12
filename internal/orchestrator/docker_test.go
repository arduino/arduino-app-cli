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

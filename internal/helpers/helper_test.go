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

package helpers

import (
	"testing"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
)

func TestArduinoCLIDownloadProgressToString(t *testing.T) {
	tests := []struct {
		name     string
		input    *rpc.DownloadProgress
		expected string
	}{
		{
			name: "Download Start",
			input: &rpc.DownloadProgress{
				Message: &rpc.DownloadProgress_Start{
					Start: &rpc.DownloadProgressStart{
						Url: "http://example.com/index.json",
					},
				},
			},
			expected: "Download started: http://example.com/index.json",
		},
		{
			name: "Download Update",
			input: &rpc.DownloadProgress{
				Message: &rpc.DownloadProgress_Update{
					Update: &rpc.DownloadProgressUpdate{
						Downloaded: 1048576,
						TotalSize:  5242880,
					},
				},
			},
			expected: "Downloading: 1.00MiB / 5.00MiB",
		},
		{
			name: "Download End Success",
			input: &rpc.DownloadProgress{
				Message: &rpc.DownloadProgress_End{
					End: &rpc.DownloadProgressEnd{
						Success: true,
						Message: "Done",
					},
				},
			},
			expected: "Download completed",
		},
		{
			name: "Download End Failure",
			input: &rpc.DownloadProgress{
				Message: &rpc.DownloadProgress_End{
					End: &rpc.DownloadProgressEnd{
						Success: false,
						Message: "Network error",
					},
				},
			},
			expected: "Download failed: Network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ArduinoCLIDownloadProgressToString(tt.input)
			if res != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, res)
			}
		})
	}
}

func TestArduinoCLITaskProgressToString(t *testing.T) {
	tests := []struct {
		name     string
		input    *rpc.TaskProgress
		expected string
	}{
		{
			name: "Task Running",
			input: &rpc.TaskProgress{
				Name:    "Install",
				Percent: 50.0,
			},
			expected: "Install: 50%",
		},
		{
			name: "Task With Message",
			input: &rpc.TaskProgress{
				Name:    "Unpacking",
				Message: "Extracting...",
				Percent: 10.0,
			},
			expected: "Unpacking (Extracting...): 10%",
		},
		{
			name: "Task Completed",
			input: &rpc.TaskProgress{
				Name:      "Install",
				Completed: true,
			},
			expected: "Install: completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ArduinoCLITaskProgressToString(tt.input)
			if res != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, res)
			}
		})
	}
}

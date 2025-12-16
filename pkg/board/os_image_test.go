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
package board

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

// implements remote.RemoteConn
type MockRemoteConn struct {
	ReadFileFunc func(path string) (io.ReadCloser, error)
}

func (m *MockRemoteConn) ReadFile(path string) (io.ReadCloser, error) {
	return m.ReadFileFunc(path)
}

// Empty definitions
func (m *MockRemoteConn) List(path string) ([]remote.FileInfo, error) {
	return nil, nil
}
func (m *MockRemoteConn) MkDirAll(path string) error {
	return nil
}
func (m *MockRemoteConn) Remove(path string) error {
	return nil
}
func (m *MockRemoteConn) Stats(path string) (remote.FileInfo, error) {
	return remote.FileInfo{}, nil
}
func (m *MockRemoteConn) WriteFile(data io.Reader, path string) error {
	return nil
}
func (m *MockRemoteConn) GetCmd(cmd string, args ...string) remote.Cmder {
	return nil
}
func (m *MockRemoteConn) Forward(ctx context.Context, localPort int, remotePort int) error {
	return nil
}
func (m *MockRemoteConn) ForwardKillAll(ctx context.Context) error {
	return nil
}
func createBuildInfoConnection(imageVersion string) remote.RemoteConn {
	mockConn := MockRemoteConn{
		ReadFileFunc: func(path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(imageVersion)), nil
		},
	}
	return &mockConn
}

func TestParseOSImageVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		found    bool
	}{
		{
			name:     "valid build id",
			input:    "BUILD_ID=20251006-395\nVARIANT_ID=xfce",
			expected: "20251006-395",
			found:    true,
		},
		{
			name:  "missing build id",
			input: "VARIANT_ID=xfce\n",
			found: false,
		},
		{
			name:  "empty build id",
			input: "BUILD_ID=\n",
			found: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseOSImageVersion(strings.NewReader(tt.input))
			if ok != tt.found || got != tt.expected {
				t.Fatalf("got (%q, %v), expected (%q, %v)", got, ok, tt.expected, tt.found)
			}
		})
	}
}

func TestGetOSImageVersion(t *testing.T) {
	const R0_IMAGE_VERSION_ID = "20250807-136"
	R0Version := createBuildInfoConnection(R0_IMAGE_VERSION_ID)
	AnotherVersion := createBuildInfoConnection("BUILD_ID=20250101-001")
	require.Equal(t, GetOSImageVersion(R0Version), R0_IMAGE_VERSION_ID)
	require.Equal(t, GetOSImageVersion(AnotherVersion), "20250101-001")
}

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package board

import (
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

func createBuildInfoConnection(imageVersion string) remote.FS {
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

func TestIsUserPartitionPreservationSupported(t *testing.T) {
	const R0_IMAGE_VERSION_ID = "20250807-136"
	anotherVersionId := "20250101-001"

	tests := []struct {
		name                    string
		currentVersion          string
		targetVersion           string
		isPreservationSupported bool
	}{
		{
			name:                    "both versions are *not* R0",
			currentVersion:          anotherVersionId,
			targetVersion:           "20250101-001",
			isPreservationSupported: true,
		},
		{
			name:                    "current version is R0",
			currentVersion:          R0_IMAGE_VERSION_ID,
			targetVersion:           "20250101-001",
			isPreservationSupported: false,
		},
		{
			name:                    "target version is R0",
			currentVersion:          anotherVersionId,
			targetVersion:           R0_IMAGE_VERSION_ID,
			isPreservationSupported: false,
		},
		{
			name:                    "both versions are R0",
			currentVersion:          R0_IMAGE_VERSION_ID,
			targetVersion:           R0_IMAGE_VERSION_ID,
			isPreservationSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isPreservationSupported := IsUserPartitionPreservationSupported(tt.currentVersion, tt.targetVersion)
			require.Equal(t, isPreservationSupported, tt.isPreservationSupported)
		})
	}
}

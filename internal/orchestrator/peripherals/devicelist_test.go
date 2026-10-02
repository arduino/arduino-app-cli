// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package peripherals

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/linuxconfig"
)

func TestCamerasFromStablePaths(t *testing.T) {
	t.Run("keeps one entry per physical camera", func(t *testing.T) {
		// A single UVC camera exposes a capture node and a metadata node.
		cameras := camerasFromStablePaths([]string{
			"/dev/v4l/by-id/usb-UGREEN_Camera_UGREEN_Camera_SN0001-video-index0",
			"/dev/v4l/by-id/usb-UGREEN_Camera_UGREEN_Camera_SN0001-video-index1",
		})

		require.Len(t, cameras, 1)
		assert.Equal(t, "usb:/dev/v4l/by-id/usb-UGREEN_Camera_UGREEN_Camera_SN0001-video-index0", cameras[0].Identifier)
		assert.Equal(t, "UGREEN Camera UGREEN Camera SN0001", cameras[0].Name)
		assert.Equal(t, CameraClass, cameras[0].Class)
		assert.Equal(t, USBFamily, cameras[0].Family)
		assert.True(t, cameras[0].Resolved)
	})

	t.Run("keeps distinct cameras apart", func(t *testing.T) {
		cameras := camerasFromStablePaths([]string{
			"/dev/v4l/by-id/usb-UGREEN_Camera_SN0001-video-index0",
			"/dev/v4l/by-id/usb-UGREEN_Camera_SN0001-video-index1",
			"/dev/v4l/by-id/usb-046d_HD_Pro_Webcam_C920_A1B2C3-video-index0",
		})

		assert.Len(t, cameras, 2)
	})

	t.Run("no camera", func(t *testing.T) {
		assert.Empty(t, camerasFromStablePaths(nil))
	})
}

func TestCameraNameFromStablePath(t *testing.T) {
	assert.Equal(t, "UGREEN Camera UGREEN Camera SN0001",
		cameraNameFromStablePath("/dev/v4l/by-id/usb-UGREEN_Camera_UGREEN_Camera_SN0001-video-index0"))
	assert.Equal(t, "046d HD Pro Webcam C920 A1B2C3",
		cameraNameFromStablePath("/dev/v4l/by-id/usb-046d_HD_Pro_Webcam_C920_A1B2C3-video-index1"))
}

func TestListCarrierDevices(t *testing.T) {
	carriers := []linuxconfig.Carrier{
		{
			CarrierName: "media-carrier",
			Devices: []linuxconfig.Device{
				{Device: "camera0", DeviceType: "camera", Option: "type1-4lanes"},
				{Device: "camera1", DeviceType: "camera", Option: "none"}, // disabled
				{Device: "display", DeviceType: "display", Option: "dsi"}, // class we do not handle
			},
		},
		{
			CarrierName: "some-other-carrier",
			Devices: []linuxconfig.Device{
				{Device: "camera0", DeviceType: "camera", Option: "type1-4lanes"},
			},
		},
	}

	devices := listCarrierDevices(carriers)

	require.Len(t, devices, 1)
	assert.Equal(t, MediaCarrierFamily, devices[0].Family)
	assert.Equal(t, CameraClass, devices[0].Class)
	assert.Equal(t, "media-carrier/camera0", devices[0].Identifier)
	assert.False(t, devices[0].Resolved, "the brick resolves carrier devices")
}

func TestAssignIndexes(t *testing.T) {
	devices := assignIndexes([]Device{
		{Family: USBFamily, Class: CameraClass, Identifier: "usb:/dev/v4l/by-id/usb-b-video-index0"},
		{Family: USBFamily, Class: CameraClass, Identifier: "usb:/dev/v4l/by-id/usb-a-video-index0"},
		{Family: MediaCarrierFamily, Class: CameraClass, Identifier: "media-carrier/camera0"},
		{Family: USBFamily, Class: MicrophoneClass, Identifier: "usb:mic"},
	})

	logicalNames := []string{}
	for _, d := range devices {
		logicalNames = append(logicalNames, d.LogicalName())
	}

	// Indexes restart per family and class, and follow the identifier order.
	assert.Equal(t, []string{
		"media-carrier/camera0",
		"usb/camera0",
		"usb/camera1",
		"usb/microphone0",
	}, logicalNames)
	assert.Equal(t, "usb:/dev/v4l/by-id/usb-a-video-index0", devices[1].Identifier)
}

func TestAssignIndexesIsStable(t *testing.T) {
	// The same hardware must always get the same logical name, whatever the
	// order the enumeration returned it in.
	first := assignIndexes([]Device{
		{Family: USBFamily, Class: CameraClass, Identifier: "usb:b"},
		{Family: USBFamily, Class: CameraClass, Identifier: "usb:a"},
	})
	second := assignIndexes([]Device{
		{Family: USBFamily, Class: CameraClass, Identifier: "usb:a"},
		{Family: USBFamily, Class: CameraClass, Identifier: "usb:b"},
	})

	assert.Equal(t, first, second)
}

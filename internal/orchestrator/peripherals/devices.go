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

package peripherals

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
)

type AvailableDevices struct {
	DevicePaths    []string
	HasVideoDevice bool
	HasSoundDevice bool
	HasGPUDevice   bool
}

type DeviceClass string

const (
	CameraClass     DeviceClass = "camera"
	MicrophoneClass DeviceClass = "microphone"
	SpeakerClass    DeviceClass = "speaker"
)

func Detect() (AvailableDevices, error) {
	res := AvailableDevices{}

	deviceList, err := paths.New("/dev").ReadDir()
	if err != nil {
		slog.Error("unable to list /dev", slog.String("error", err.Error()))
		return AvailableDevices{}, fmt.Errorf("unable to list board devices")
	}

	for _, p := range deviceList {
		switch {
		case p.HasPrefix("video"):
			res.DevicePaths = append(res.DevicePaths, p.String())
		case p.HasPrefix("dri"):
			res.HasGPUDevice = true
		}
	}

	// Verify if there are real video devices (cameras) in /dev/v4l/by-id
	if camDevices := GetVideoDevices(); len(camDevices) > 0 {
		res.HasVideoDevice = true
	}
	// Verify if there are real sound devices in /dev/snd/by-id
	if sndDev := GetSoundDevices(); len(sndDev) > 0 {
		res.DevicePaths = append(res.DevicePaths, "/dev/snd")
		res.HasSoundDevice = true
	}
	// Verify if we need to add GPU devices
	if res.HasGPUDevice {
		res.DevicePaths = append(res.DevicePaths, "/dev/dri")
	}

	return res, nil
}

func GetSoundDevices() []string {
	// Check and read /dev/snd. This fs contains only real sound devices
	soundDevicePath := paths.New("/dev/snd/by-id")
	if _, err := soundDevicePath.Stat(); err != nil {
		return nil // no sound device found
	}
	sndDeviceList, err := soundDevicePath.ReadDir()
	if err != nil {
		slog.Warn("unable to list /dev/snd/by-id", slog.String("error", err.Error()))
		return nil
	}
	detectedDevices := []string{}
	for _, sndD := range sndDeviceList {
		detectedDevices = append(detectedDevices, sndD.String())
	}
	return detectedDevices
}

func GetVideoDevices() map[int]string {
	// Check and read /dev/v4l/by-id. This fs contains only real video devices (cameras), filtering out devices for HW acceleration (like Qualcomm Venus)
	videoDevicePath := paths.New("/dev/v4l/by-id")
	if _, err := videoDevicePath.Stat(); err != nil {
		return nil // no video device found
	}
	v4DeviceList, err := videoDevicePath.ReadDir()
	if err != nil {
		slog.Warn("unable to list /dev/v4l/by-id", slog.String("error", err.Error()))
		return nil
	}
	sortedDevices := []string{}
	for _, v4d := range v4DeviceList {
		sortedDevices = append(sortedDevices, v4d.String())
	}
	sortV4lByIndexDevices(sortedDevices)

	camDevices := []string{}
	for _, v4d := range sortedDevices {
		if linked, err := os.Readlink(v4d); err == nil {
			split := strings.Split(linked, "/")
			realVideoDev := filepath.Join("/dev", split[len(split)-1])
			slog.Debug("found v4l device", slog.String("device", v4d), slog.String("linked", linked), slog.String("realDevice", realVideoDev))
			camDevices = append(camDevices, realVideoDev)
		} else {
			slog.Warn("unable to readlink v4l device", slog.String("device", v4d), slog.String("error", err.Error()))
		}
	}
	// VIDEO_DEVICE will be the first device in /dev/v4l/by-id
	slog.Debug("sorted camera devices", slog.Any("devices", camDevices))
	deviceMap := map[int]string{}
	for i, cam := range camDevices {
		slog.Debug("found camera device", slog.Int("index", i), slog.String("device", cam))
		deviceMap[i] = cam
	}
	return deviceMap
}

func sortV4lByIndexDevices(deviceList []string) {
	slices.SortFunc(deviceList, func(a, b string) int {
		// Extract the index from the first string
		indexI, err := extractIndexFromVideoDeviceName(a)
		if err != nil {
			return 0
		}

		// Extract the index from the second string
		indexJ, err := extractIndexFromVideoDeviceName(b)
		if err != nil {
			return 0
		}

		// Compare the numeric indices
		switch {
		case indexI < indexJ:
			return -1
		case indexI > indexJ:
			return 1
		default:
			return 0
		}
	})
}

func extractIndexFromVideoDeviceName(device string) (int, error) {
	idx := strings.LastIndex(device, "index")

	if idx == -1 {
		return -1, fmt.Errorf("substring 'index' not found in %q", device)
	}

	start := idx + len("index")
	dev := device[start:]

	return strconv.Atoi(dev)
}

func HasVirtualDevice(deviceClass DeviceClass, devices []string) bool {
	virtualDevicesMapping := map[DeviceClass][]string{
		CameraClass: {"remote_camera_0"},
	}

	for _, v := range virtualDevicesMapping[deviceClass] {
		for _, d := range devices {
			if v == d {
				return true
			}
		}
	}
	return false
}

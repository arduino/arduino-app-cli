// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package peripherals

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/linuxconfig"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type AvailableDevices struct {
	HasVideoDevice        bool
	HasSoundDevice        bool
	HasGPUDevice          bool
	HasCSICameraDevice    bool
	HasCarrierSoundDevice bool
}

type DeviceClass string

const (
	CameraClass     DeviceClass = "camera"
	MicrophoneClass DeviceClass = "microphone"
	SpeakerClass    DeviceClass = "speaker"
)

func Detect(ctx context.Context, plat platform.Platform) (AvailableDevices, error) {
	devices := AvailableDevices{}

	deviceList, err := paths.New("/dev").ReadDir()
	if err != nil {
		slog.Error("unable to list /dev", slog.String("error", err.Error()))
		return AvailableDevices{}, fmt.Errorf("unable to list board devices")
	}

	for _, p := range deviceList {
		if p.HasPrefix("dri") {
			devices.HasGPUDevice = true
		}
	}

	// Verify if there are real video devices (cameras) in /dev/v4l/by-id
	if camDevices := GetVideoDevices(); len(camDevices) > 0 {
		devices.HasVideoDevice = true
	}
	// Verify if there are real sound devices in /dev/snd/by-id
	if sndDev := GetSoundDevices(); sndDev > 0 {
		devices.HasSoundDevice = true
	}

	carriers, err := linuxconfig.GetEnabledCarriers(ctx)
	if err != nil {
		slog.Warn("unable to get enabled devices from linux config", slog.String("error", err.Error()))
	}
	devices.HasCarrierSoundDevice = HasSoundDeviceOnCarrier(carriers)
	devices.HasCSICameraDevice = HasCSICameraDriver() && (plat.HasNativeCSICameraSupport() || HasCSICameraOnCarrier(carriers))

	return devices, nil
}

func GetSoundDevices() int {
	// Check and read /dev/snd. This fs contains only real sound devices
	soundDevicePath := paths.New("/dev/snd/by-id")
	if _, err := soundDevicePath.Stat(); err != nil {
		return 0 // no sound device found
	}
	sndDeviceList, err := soundDevicePath.ReadDir()
	if err != nil {
		slog.Warn("unable to list /dev/snd/by-id", slog.String("error", err.Error()))
		return 0
	}
	return len(sndDeviceList)
}

// VideoDevice holds both identifiers of a camera:
//   - StablePath is the /dev/v4l/by-id symlink, built by udev from the device
//     attributes, so it survives a reboot or a change of USB port.
//   - DevPath is the /dev/videoN node it points to, whose number depends on the
//     enumeration order and can change at every boot.
type VideoDevice struct {
	StablePath string
	DevPath    string
}

// GetVideoDeviceList returns the cameras found in /dev/v4l/by-id, in stable order,
// keeping both identifiers.
func GetVideoDeviceList() []VideoDevice {
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

	camDevices := []VideoDevice{}
	for _, v4d := range sortedDevices {
		if linked, err := os.Readlink(v4d); err == nil {
			split := strings.Split(linked, "/")
			realVideoDev := filepath.Join("/dev", split[len(split)-1])
			slog.Debug("found v4l device", slog.String("device", v4d), slog.String("linked", linked), slog.String("realDevice", realVideoDev))
			camDevices = append(camDevices, VideoDevice{StablePath: v4d, DevPath: realVideoDev})
		} else {
			slog.Warn("unable to readlink v4l device", slog.String("device", v4d), slog.String("error", err.Error()))
		}
	}
	return camDevices
}

func GetVideoDevices() map[int]string {
	camDevices := GetVideoDeviceList()
	// VIDEO_DEVICE will be the first device in /dev/v4l/by-id
	slog.Debug("sorted camera devices", slog.Any("devices", camDevices))
	deviceMap := map[int]string{}
	for i, cam := range camDevices {
		slog.Debug("found camera device", slog.Int("index", i), slog.String("device", cam.DevPath))
		deviceMap[i] = cam.DevPath
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

type CSICameraDriver string

const (
	CSICameraDriverNone  CSICameraDriver = ""
	CSICameraDriverCamss CSICameraDriver = "camss"
	CSICameraDriverCamx  CSICameraDriver = "camx"
)

// HasCSICameraDriver reports if a CSI camera driver is bound on the board
func HasCSICameraDriver() bool {
	return DetectCSICameraDriver() != CSICameraDriverNone
}

// DetectCSICameraDriver returns the CSI camera driver available on the board, if any
func DetectCSICameraDriver() CSICameraDriver {
	return detectCSICameraDriver("/sys/bus/platform/drivers", "/run/cam_server/le_cam_socket")
}

func detectCSICameraDriver(driversDir string, camxSocket string) CSICameraDriver {
	// Detect camss
	if _, err := os.Stat(filepath.Join(driversDir, "qcom-camss")); err == nil {
		slog.Debug("detected camss CSI camera driver")
		return CSICameraDriverCamss
	}

	// Detect camx
	entries, err := os.ReadDir(driversDir)
	if err != nil {
		slog.Debug("unable to list platform drivers", slog.String("dir", driversDir), slog.String("error", err.Error()))
		return CSICameraDriverNone
	}
	hasCamxDriver := slices.ContainsFunc(entries, func(e os.DirEntry) bool {
		return e.IsDir() && strings.HasPrefix(e.Name(), "cam_")
	})
	if !hasCamxDriver {
		return CSICameraDriverNone
	}
	if _, err := os.Stat(camxSocket); err != nil {
		return CSICameraDriverNone
	}
	slog.Debug("detected camx CSI camera driver")

	return CSICameraDriverCamx
}

// HasSoundDeviceOnCarrier reports if an enabled media carrier provides a sound device
func HasSoundDeviceOnCarrier(carriers []linuxconfig.Carrier) bool {
	return slices.ContainsFunc(carriers, func(c linuxconfig.Carrier) bool { return c.CarrierName == "media-carrier" })
}

// HasCSICameraOnCarrier reports if a CSI camera is configured on an enabled media carrier
func HasCSICameraOnCarrier(carriers []linuxconfig.Carrier) bool {
	for _, c := range carriers {
		if c.CarrierName != "media-carrier" {
			continue
		}
		if slices.ContainsFunc(c.EnabledDevices(), func(d linuxconfig.Device) bool { return strings.Contains(d.Device, "camera") }) {
			return true
		}
	}
	return false
}

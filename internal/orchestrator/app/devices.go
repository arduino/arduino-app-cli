package app

import (
	"errors"
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
	hasGPUDevice   bool
}

func GetAvailableDevices() (*AvailableDevices, error) {
	res := AvailableDevices{}

	deviceList, err := paths.New("/dev").ReadDir()
	if err != nil {
		slog.Error("unable to list /dev", slog.String("error", err.Error()))
		return nil, fmt.Errorf("unable to list board devices")
	}

	for _, p := range deviceList {
		switch {
		case p.HasPrefix("video"):
			res.DevicePaths = append(res.DevicePaths, p.String())
		case p.HasPrefix("dri"):
			res.hasGPUDevice = true
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
	if res.hasGPUDevice {
		res.DevicePaths = append(res.DevicePaths, "/dev/dri")
	}

	return &res, nil
}

const (
	CameraDevice     = "camera"
	MicrophoneDevice = "microphone"
	SpeakerDevice    = "speaker"
)

func ValidateRequiredDevices(a AppDescriptor, res *AvailableDevices) error {
	// Required devices can be defined in both the bricks and the app descriptor
	requiredDeviceClasses := make(map[string]bool)

	if len(a.RequiredDevices) > 0 {
		for _, deviceClass := range a.RequiredDevices {
			requiredDeviceClasses[deviceClass] = true
		}
	}

	for _, brick := range a.Bricks {
		if len(brick.Devices) > 0 {
			for _, deviceClass := range brick.Devices {
				// Do not require a "camera" class if the brick in the app requires a "remote camera" device
				if deviceClass == CameraDevice && slices.Contains(brick.Devices, "remote_camera_0") {
					continue
				}
				requiredDeviceClasses[deviceClass] = true
			}
		}
	}

	var allErrors error
	if len(requiredDeviceClasses) > 0 {
		for class := range requiredDeviceClasses {
			switch class {
			case CameraDevice:
				if !res.HasVideoDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no camera device found"))
				}
			case MicrophoneDevice:
				if !res.HasSoundDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no microphone device found"))
				}
			case SpeakerDevice:
				if !res.HasSoundDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no speaker device found"))
				}
			default:
				slog.Debug("not handled device class - no action", slog.String("class", class))
			}
		}
	}

	return allErrors
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
	dev := device[strings.LastIndex(device, "index")+len("index"):]
	if indexI, err := strconv.Atoi(dev); err != nil {
		return -1, err
	} else {
		return indexI, nil
	}
}

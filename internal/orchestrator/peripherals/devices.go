// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package peripherals

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"

	linuxconfig "github.com/arduino/arduino-app-cli/internal/orchestrator/linuxConfig"
)

const pwDumpTool = "pw-dump"

const (
	bluez5DeviceAPI  = "bluez5"
	audioSinkClass   = "Audio/Sink"
	audioSourceClass = "Audio/Source"
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

func Detect(ctx context.Context) (AvailableDevices, error) {
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
	// Verify if there is a bluetooth speaker or microphone (headset) connected through PipeWire
	if !devices.HasSoundDevice {
		if nodes, ok := pwDumpNodes(ctx); ok &&
			(hasBluetoothAudioNode(nodes, audioSinkClass) || hasBluetoothAudioNode(nodes, audioSourceClass)) {
			devices.HasSoundDevice = true
		}
	}

	carriers, err := linuxconfig.GetEnabledCarriers(ctx)
	if err != nil {
		slog.Warn("unable to get enabled devices from linux config", slog.String("error", err.Error()))
	}
	devices = HasCarrierDevices(carriers, devices)

	return devices, nil
}

// pwNode represents the subset of a PipeWire object (as reported by pw-dump)
// needed to detect bluetooth audio devices (speakers and microphones).
type pwNode struct {
	Info struct {
		Props struct {
			MediaClass string `json:"media.class"`
			DeviceAPI  string `json:"device.api"`
			NodeName   string `json:"node.name"`
		} `json:"props"`
	} `json:"info"`
}

// pwDumpNodes runs pw-dump and returns the parsed PipeWire graph. The second
// return value is false when pw-dump is unavailable or fails, so callers can
// safely skip bluetooth detection.
func pwDumpNodes(ctx context.Context) ([]pwNode, bool) {
	if _, err := exec.LookPath(pwDumpTool); err != nil {
		slog.Debug("pw-dump tool not found in PATH, skipping bluetooth audio detection", slog.String("error", err.Error()))
		return nil, false
	}

	cmd, err := paths.NewProcess(nil, pwDumpTool)
	if err != nil {
		slog.Warn("unable to create pw-dump process", slog.String("error", err.Error()))
		return nil, false
	}

	stdout, stderr, err := cmd.RunAndCaptureOutput(ctx)
	if err != nil {
		slog.Warn("unable to run pw-dump", slog.String("error", err.Error()), slog.String("stderr", string(stderr)))
		return nil, false
	}

	var nodes []pwNode
	if err := json.Unmarshal(stdout, &nodes); err != nil {
		slog.Warn("unable to parse pw-dump output", slog.String("error", err.Error()))
		return nil, false
	}
	return nodes, true
}

// hasBluetoothAudioNode reports whether nodes contains a bluez5 device with the
// given media class (audioSinkClass for a speaker, audioSourceClass for a
// microphone).
func hasBluetoothAudioNode(nodes []pwNode, mediaClass string) bool {
	for _, n := range nodes {
		if n.Info.Props.DeviceAPI == bluez5DeviceAPI && n.Info.Props.MediaClass == mediaClass {
			slog.Debug("found bluetooth audio device", slog.String("node", n.Info.Props.NodeName), slog.String("mediaClass", mediaClass))
			return true
		}
	}
	return false
}

// HasBluetoothSpeaker reports whether a bluetooth speaker (a bluez5 Audio/Sink)
// is currently connected, by inspecting the PipeWire graph via pw-dump. If
// pw-dump is not available it returns false.
func HasBluetoothSpeaker(ctx context.Context) bool {
	nodes, ok := pwDumpNodes(ctx)
	return ok && hasBluetoothAudioNode(nodes, audioSinkClass)
}

// HasBluetoothMicrophone reports whether a bluetooth microphone (a bluez5
// Audio/Source, as exposed by a headset) is currently connected, by inspecting
// the PipeWire graph via pw-dump. If pw-dump is not available it returns false.
func HasBluetoothMicrophone(ctx context.Context) bool {
	nodes, ok := pwDumpNodes(ctx)
	return ok && hasBluetoothAudioNode(nodes, audioSourceClass)
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

func HasCarrierDevices(carriers []linuxconfig.Carrier, devices AvailableDevices) AvailableDevices {

	for _, c := range carriers {
		if c.CarrierName == "media-carrier" {
			devices.HasCarrierSoundDevice = true
			if slices.ContainsFunc(c.EnabledDevices(), func(d linuxconfig.Device) bool { return strings.Contains(d.Device, "camera") }) {
				devices.HasCSICameraDevice = true
				return devices
			}
		}

	}
	return devices
}

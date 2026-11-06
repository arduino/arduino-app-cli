// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package peripherals

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/linuxconfig"
)

// Family tells where a device comes from. Together with the class and an index it
// forms the logical name used in the app.yaml, e.g. "usb/camera0".
type Family string

const (
	USBFamily          Family = "usb"
	MediaCarrierFamily Family = "media-carrier"
)

const mediaCarrierName = "media-carrier"

// Device is a single peripheral available on the board.
//
// It carries two identifiers on purpose:
//   - LogicalName is what the user writes in the app.yaml. It names a role
//     ("the first USB camera"), not a physical unit, so it stays valid on another board.
//   - Identifier is what we send to the brick. When Resolved is true it is a concrete
//     value the brick can use as-is; when it is false the brick has to resolve it,
//     because the discovery only exists inside the container (see the carrier cameras).
type Device struct {
	Family     Family
	Class      DeviceClass
	Index      int
	Name       string
	Identifier string
	Resolved   bool
}

// LogicalName is the identifier used in the app.yaml: <family>/<class><index>.
func (d Device) LogicalName() string {
	return fmt.Sprintf("%s/%s%d", d.Family, d.Class, d.Index)
}

// ListDevices returns every device we can currently enumerate, in a stable order,
// with its logical index already assigned.
//
// Audio is missing on purpose: /dev/snd/by-id only covers USB cards, and the SoC card
// exposes generic "MultiMediaN" endpoints that cannot be told apart without PipeWire.
// It will be added as one more family once pw-dump is available.
func ListDevices(ctx context.Context) []Device {
	devices := listUSBCameras()

	carriers, err := linuxconfig.GetEnabledCarriers(ctx)
	if err != nil {
		slog.Warn("unable to get enabled carriers", slog.String("error", err.Error()))
	} else {
		devices = append(devices, listCarrierDevices(carriers)...)
	}

	return assignIndexes(devices)
}

// listUSBCameras enumerates the cameras exposed under /dev/v4l/by-id.
//
// A single camera usually exposes more than one V4L node (capture plus metadata), all
// sharing the same by-id prefix. We keep the lowest one, which is the capture node on
// every UVC device we have seen. Reading the V4L2 capabilities would be exact, but it
// needs an ioctl on each node.
func listUSBCameras() []Device {
	stablePaths := []string{}
	for _, cam := range GetVideoDeviceList() {
		stablePaths = append(stablePaths, cam.StablePath)
	}
	return camerasFromStablePaths(stablePaths)
}

var v4lNodeSuffix = regexp.MustCompile(`-video-index\d+$`)

func camerasFromStablePaths(stablePaths []string) []Device {
	// Group the nodes of the same physical camera together, keeping the lowest one.
	firstNodeOf := map[string]string{}
	for _, path := range stablePaths {
		key := v4lNodeSuffix.ReplaceAllString(path, "")
		if current, seen := firstNodeOf[key]; !seen || path < current {
			firstNodeOf[key] = path
		}
	}

	devices := []Device{}
	for _, path := range firstNodeOf {
		devices = append(devices, Device{
			Family:     USBFamily,
			Class:      CameraClass,
			Name:       cameraNameFromStablePath(path),
			Identifier: string(USBFamily) + ":" + path,
			Resolved:   true,
		})
	}
	return devices
}

// cameraNameFromStablePath turns the by-id file name into something readable:
// "usb-UGREEN_Camera_UGREEN_Camera_SN0001-video-index0" becomes "UGREEN Camera UGREEN Camera SN0001".
func cameraNameFromStablePath(stablePath string) string {
	name := stablePath[strings.LastIndex(stablePath, "/")+1:]
	name = strings.TrimPrefix(name, "usb-")
	name = v4lNodeSuffix.ReplaceAllString(name, "")
	return strings.ReplaceAll(name, "_", " ")
}

// listCarrierDevices reports the devices enabled on a carrier.
//
// They are passed through unresolved: reaching the concrete name means walking from the
// carrier slot to the sensor I2C address and then to the GStreamer device name, and that
// discovery lives in the brick, which is the only side with GStreamer available.
func listCarrierDevices(carriers []linuxconfig.Carrier) []Device {
	devices := []Device{}
	for _, carrier := range carriers {
		if carrier.CarrierName != mediaCarrierName {
			continue
		}
		for _, device := range carrier.EnabledDevices() {
			class := DeviceClass(device.DeviceType)
			if !slices.Contains(knownClasses, class) {
				slog.Debug("skipping carrier device of unknown class",
					slog.String("device", device.Device),
					slog.String("class", device.DeviceType))
				continue
			}
			devices = append(devices, Device{
				Family:     MediaCarrierFamily,
				Class:      class,
				Name:       device.Device,
				Identifier: string(MediaCarrierFamily) + "/" + device.Device,
				Resolved:   false,
			})
		}
	}
	return devices
}

var knownClasses = []DeviceClass{CameraClass, MicrophoneClass, SpeakerClass}

// assignIndexes sorts the devices so that the same hardware always gets the same logical
// name, then numbers them within their family and class.
//
// The order comes from the identifier, which is stable across reboots and USB ports. Note
// that it is not stable across hardware changes: unplugging the first camera makes the
// second one become "usb/camera0".
func assignIndexes(devices []Device) []Device {
	slices.SortFunc(devices, func(a, b Device) int {
		if a.Family != b.Family {
			return strings.Compare(string(a.Family), string(b.Family))
		}
		if a.Class != b.Class {
			return strings.Compare(string(a.Class), string(b.Class))
		}
		return strings.Compare(a.Identifier, b.Identifier)
	})

	counters := map[string]int{}
	for i := range devices {
		key := string(devices[i].Family) + "/" + string(devices[i].Class)
		devices[i].Index = counters[key]
		counters[key]++
	}
	return devices
}

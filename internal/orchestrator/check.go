// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
)

// checkBricks validates that each app brick exists in the index, that its selected model (when
// required) is installed, and that all required brick variables are set.
// Errors are joined so every issue is reported at once.
func checkBricks(ctx context.Context, bricks []app.Brick, index *bricksindex.BricksIndex, modelIndex *modelsindex.ModelsIndex) error {
	var allErrors error
	for _, appBrick := range bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			allErrors = errors.Join(allErrors, fmt.Errorf("brick %q not found", appBrick.ID))
			continue // Skip further validation for this brick since it doesn't exist
		}

		if indexBrick.RequireModel {
			selectedModel := cmp.Or(appBrick.Model, indexBrick.ModelName)
			model, err := modelIndex.GetModelByID(ctx, selectedModel)
			switch {
			case err != nil:
				allErrors = errors.Join(allErrors, fmt.Errorf("retrieving model %q for brick %q: %w", selectedModel, appBrick.ID, err))
			case model == nil:
				allErrors = errors.Join(allErrors, fmt.Errorf("model %q for brick %q not found", selectedModel, appBrick.ID))
			default:
				if model.Status != modelsindex.InstalledStatus {
					allErrors = errors.Join(allErrors, fmt.Errorf("model %q for brick %q is not installed", selectedModel, appBrick.ID))
				}
				if !modelIndex.IsModelSupportedByBrick(selectedModel, appBrick.ID) {
					allErrors = errors.Join(allErrors, fmt.Errorf("model %q is not compatible with brick %q", selectedModel, appBrick.ID))
				}
			}
		}

		for appBrickVariableName := range appBrick.Variables {
			_, exist := indexBrick.GetVariable(appBrickVariableName)
			if !exist {
				// TODO: we should return warnings
				slog.Warn("[skip] variable does not exist into the brick definition", "variable", appBrickVariableName, "brick", indexBrick.ID)
			}
		}

		// Check that all required brick variables are provided by app
		for _, indexBrickVariable := range indexBrick.Variables {
			if indexBrickVariable.IsRequired() {
				if _, exist := appBrick.Variables[indexBrickVariable.Name]; !exist {
					allErrors = errors.Join(allErrors, fmt.Errorf("variable %q is required by brick %q", indexBrickVariable.Name, indexBrick.ID))
				}
			}
		}
	}

	return allErrors
}

// appPortsSource is the collision source name used for the ports declared in the app.yaml file.
const appPortsSource = "app.yaml"

// RequiredService is a singleton service pulled in by an app brick. The brick that required it is
// kept around because services are not a concept exposed to the user: ports and collisions are
// reported against the brick, not against the service.
type RequiredService struct {
	ID      string
	Name    string
	BrickID string
	Ports   []string
}

// RequiredServices returns the services required by the given app bricks, deduplicated by service
// ID and sorted by it. A service can be required by more than one brick, but it is started once and
// publishes its ports once, so it must count as a single source: the first brick requiring it is
// the one it is reported against.
// Bricks missing from the index, and services not available for the current board, are skipped.
func RequiredServices(
	bricks []app.Brick,
	index *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
) ([]RequiredService, error) {
	services := make(map[string]RequiredService)
	for _, appBrick := range bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			continue
		}

		matchingServices, err := indexBrick.GetMatchingService(bricksindex.BrickInstance{
			Model: cmp.Or(appBrick.Model, indexBrick.ModelName),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get required services for brick %s: %w", appBrick.ID, err)
		}

		for _, id := range matchingServices {
			if _, alreadyRequired := services[id]; alreadyRequired {
				continue
			}
			service, found := servicesIndex.FindServiceByID(id)
			if !found {
				slog.Debug("service required by brick not found or not available for current board", slog.String("service_id", id), slog.String("brick_id", appBrick.ID))
				continue
			}
			services[id] = RequiredService{ID: id, Name: service.Name, BrickID: appBrick.ID, Ports: service.GetPorts()}
		}
	}

	return slices.SortedFunc(maps.Values(services), func(a, b RequiredService) int {
		return cmp.Compare(a.ID, b.ID)
	}), nil
}

// checkPortCollisions validates that no port is declared by more than one source of the app: the
// ports field of app.yaml, the brick index, the brick compose files, and the compose files of the
// services required by the bricks.
// Errors are joined so that every collision is reported at once.
func checkPortCollisions(
	descriptor app.AppDescriptor,
	index *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
) error {
	sourcesByPort := make(map[string][]string)
	addSource := func(port, source string) {
		if !slices.Contains(sourcesByPort[port], source) {
			sourcesByPort[port] = append(sourcesByPort[port], source)
		}
	}

	for _, appBrick := range descriptor.Bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			continue
		}

		for _, p := range indexBrick.GetPorts() {
			addSource(p, appBrick.ID)
		}
	}

	services, err := RequiredServices(descriptor.Bricks, index, servicesIndex)
	if err != nil {
		return err
	}
	for _, service := range services {
		for _, p := range service.Ports {
			addSource(p, fmt.Sprintf("%s (service %s)", service.BrickID, service.Name))
		}
	}

	for _, p := range descriptor.Ports {
		addSource(strconv.Itoa(p), appPortsSource)
	}

	var allErrors error
	for _, p := range slices.Sorted(maps.Keys(sourcesByPort)) {
		if len(sourcesByPort[p]) > 1 {
			allErrors = errors.Join(allErrors, fmt.Errorf(
				"port %s is declared by multiple sources: %s", p, strings.Join(sourcesByPort[p], ", "),
			))
		}
	}

	return allErrors
}

// requiredDeviceClasses returns the sorted device classes required by the app bricks, skipping the
// ones satisfied by a virtual device.
func requiredDeviceClasses(bricksIndex *bricksindex.BricksIndex, appBricks []app.Brick) ([]peripherals.DeviceClass, error) {
	required := make(map[peripherals.DeviceClass]bool)

	for _, brick := range appBricks {
		idxBrick, found := bricksIndex.FindBrickByID(brick.ID)
		if !found {
			return nil, fmt.Errorf("brick %q not found", brick.ID)
		}

		// skip checks for virtual devices
		for _, deviceClass := range idxBrick.RequiredDevices {
			if peripherals.HasVirtualDevice(deviceClass, brick.Devices) {
				continue
			}
			required[deviceClass] = true
		}
	}

	return slices.Sorted(maps.Keys(required)), nil
}

// needsAudioDevices reports whether the app requires a microphone or a speaker.
func needsAudioDevices(requiredClasses []peripherals.DeviceClass) bool {
	return slices.Contains(requiredClasses, peripherals.MicrophoneClass) ||
		slices.Contains(requiredClasses, peripherals.SpeakerClass)
}

func checkRequiredDevices(requiredClasses []peripherals.DeviceClass, availableDevices peripherals.AvailableDevices) error {
	var allErrors error
	for _, class := range requiredClasses {
		switch class {
		case peripherals.CameraClass:
			if !availableDevices.HasVideoDevice && !availableDevices.HasCSICameraDevice {
				allErrors = errors.Join(allErrors, fmt.Errorf("no camera device found"))
			}
		//TODO: not all profile in the media carrier have a mic.
		case peripherals.MicrophoneClass:
			if !availableDevices.HasSoundDevice && !availableDevices.HasCarrierSoundDevice {
				allErrors = errors.Join(allErrors, fmt.Errorf("no microphone device found"))
			}
		case peripherals.SpeakerClass:
			if !availableDevices.HasSoundDevice && !availableDevices.HasCarrierSoundDevice {
				allErrors = errors.Join(allErrors, fmt.Errorf("no speaker device found"))
			}
		default:
			slog.Debug("not handled device class - no action", slog.String("class", string(class)))
		}
	}

	return allErrors
}

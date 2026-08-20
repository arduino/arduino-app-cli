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

// portCollision is a port declared by more than one source.
type portCollision struct {
	Port    string
	Sources []string
}

func detectPortCollisions(
	appPorts []int,
	bricks []app.Brick,
	index *bricksindex.BricksIndex,
	servicePorts map[string][]string,
) []portCollision {
	sourcesByPort := make(map[string][]string)
	addSource := func(port, source string) {
		if !slices.Contains(sourcesByPort[port], source) {
			sourcesByPort[port] = append(sourcesByPort[port], source)
		}
	}

	for _, p := range appPorts {
		addSource(strconv.Itoa(p), appPortsSource)
	}

	for _, appBrick := range bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			continue
		}

		for _, p := range indexBrick.GetPorts() {
			addSource(p, appBrick.ID)
		}
	}

	for _, id := range slices.Sorted(maps.Keys(servicePorts)) {
		for _, p := range servicePorts[id] {
			addSource(p, id)
		}
	}

	var collisions []portCollision
	for _, p := range slices.Sorted(maps.Keys(sourcesByPort)) {
		if len(sourcesByPort[p]) > 1 {
			collisions = append(collisions, portCollision{Port: p, Sources: sourcesByPort[p]})
		}
	}

	return collisions
}

// checkPortCollisions validates that no port is declared by more than one source of the app.
// Errors are joined so every collision is reported at once.
func checkPortCollisions(
	appPorts []int,
	bricks []app.Brick,
	index *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
) error {
	services, err := requiredServices(index, servicesIndex, bricks)
	if err != nil {
		return err
	}

	servicePorts := make(map[string][]string, len(services))
	for id, service := range services {
		servicePorts[id] = service.GetPorts()
	}

	var allErrors error
	for _, collision := range detectPortCollisions(appPorts, bricks, index, servicePorts) {
		slog.Error("port collision detected", slog.String("port", collision.Port), slog.Any("sources", collision.Sources))
		allErrors = errors.Join(allErrors, fmt.Errorf(
			"port %s is declared by more than one source (%s)", collision.Port, strings.Join(collision.Sources, ", "),
		))
	}

	return allErrors
}

func requiredServices(
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	appBricks []app.Brick,
) (map[string]servicesindex.Service, error) {
	services := make(map[string]servicesindex.Service)
	for _, appBrick := range appBricks {
		indexBrick, found := bricksIndex.FindBrickByID(appBrick.ID)
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
			service, found := servicesIndex.FindServiceByID(id)
			if !found {
				slog.Debug("service required by brick not found or not available for current board", slog.String("service_id", id), slog.String("brick_id", appBrick.ID))
				continue
			}
			services[id] = *service
		}
	}

	return services, nil
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

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

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
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

func checkRequiredDevices(bricksIndex *bricksindex.BricksIndex, appBricks []app.Brick, availableDevices peripherals.AvailableDevices) error {
	requiredDeviceClasses := make(map[peripherals.DeviceClass]bool)

	for _, brick := range appBricks {
		idxBrick, found := bricksIndex.FindBrickByID(brick.ID)
		if !found {
			slog.Warn("Cannot validate required devices. Brick not found", slog.String("brick_id", brick.ID))
			continue
		}

		// skip checks for virtual devices
		for _, deviceClass := range idxBrick.RequiredDevices {
			if peripherals.HasVirtualDevice(deviceClass, brick.Devices) {
				continue
			}
			requiredDeviceClasses[deviceClass] = true
		}
	}

	var allErrors error
	devices := slices.Sorted(maps.Keys(requiredDeviceClasses))
	if len(devices) > 0 {
		for _, class := range devices {
			switch class {
			case peripherals.CameraClass:
				if !availableDevices.HasVideoDevice && !availableDevices.HasCSICameraDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no camera device found"))
				}
			case peripherals.MicrophoneClass:
				if !availableDevices.HasSoundDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no microphone device found"))
				}
			case peripherals.SpeakerClass:
				if !availableDevices.HasSoundDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no speaker device found"))
				}
			default:
				slog.Debug("not handled device class - no action", slog.String("class", string(class)))
			}
		}
	}

	return allErrors
}

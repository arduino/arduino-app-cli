package orchestrator

import (
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

// CheckBricks checks that all bricks referenced in the given AppDescriptor exist in the provided BricksIndex,
// It collects and returns all validation errors as a single joined error, allowing the caller to see all issues at once rather than stopping at the first error.
func checkBricks(a app.AppDescriptor, index *bricksindex.BricksIndex, modelIndex *modelsindex.ModelsIndex) error {
	if index == nil {
		return fmt.Errorf("bricks index cannot be nil")
	}
	if modelIndex == nil {
		return fmt.Errorf("model index cannot be nil")
	}

	var allErrors error

	for _, appBrick := range a.Bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			allErrors = errors.Join(allErrors, fmt.Errorf("brick %q not found", appBrick.ID))
			continue // Skip further validation for this brick since it doesn't exist
		}

		if len(appBrick.Model) != 0 {
			_, modelFound := modelIndex.GetModelByID(appBrick.Model)
			if !modelFound {
				allErrors = errors.Join(allErrors, fmt.Errorf("model %q for brick %q not found", appBrick.Model, appBrick.ID))
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

const (
	CameraDevice     = "camera"
	MicrophoneDevice = "microphone"
	SpeakerDevice    = "speaker"
)

func checkRequiredDevices(bricksIndex *bricksindex.BricksIndex, appBricks []app.Brick, availableDevices peripherals.AvailableDevices) error {
	requiredDeviceClasses := make(map[string]bool)

	for _, brick := range appBricks {
		idxBrick, found := bricksIndex.FindBrickByID(brick.ID)
		if !found {
			slog.Warn("Cannot validate required devices. Brick not found", slog.String("brick_id", brick.ID))
			continue
		}

		for _, deviceClass := range idxBrick.RequiredDevices {
			// Do not require a "camera" class if the brick in the app requires a "remote camera" device
			if deviceClass == CameraDevice && slices.Contains(brick.Devices, "remote_camera_0") {
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
			case CameraDevice:
				if !availableDevices.HasVideoDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no camera device found"))
				}
			case MicrophoneDevice:
				if !availableDevices.HasSoundDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no microphone device found"))
				}
			case SpeakerDevice:
				if !availableDevices.HasSoundDevice {
					allErrors = errors.Join(allErrors, fmt.Errorf("no speaker device found"))
				}
			default:
				slog.Debug("not handled device class - no action", slog.String("class", class))
			}
		}
	}

	return allErrors
}

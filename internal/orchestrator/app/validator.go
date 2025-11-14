package app

import (
	"fmt"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

// ValidateBricks checks that all bricks referenced in the given AppDescriptor exist in the provided BricksIndex,
// and that all required variables for each brick are present and valid. It returns an error if any brick is missing,
// if any variable referenced by the app does not exist in the corresponding brick, or if any required variable is missing.
// If the index is nil, validation is skipped and nil is returned.
func ValidateBricks(a AppDescriptor, index *bricksindex.BricksIndex) error {
	if index == nil {
		return nil
	}
	for _, appBrick := range a.Bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			return fmt.Errorf("brick %q not found", appBrick.ID)
		}

		for appBrickName := range appBrick.Variables {
			_, exist := indexBrick.GetVariable(appBrickName)
			if !exist {
				return fmt.Errorf("variable %q does not exist on brick %q", appBrickName, indexBrick.ID)
			}
		}

		for _, indexBrickVariable := range indexBrick.Variables {
			if indexBrickVariable.IsRequired() {
				if _, exist := appBrick.Variables[indexBrickVariable.Name]; !exist {
					return fmt.Errorf("variable %q is required by brick %q", indexBrickVariable.Name, indexBrick.ID)
				}
			}
		}
	}
	return nil
}

package app

import (
	"fmt"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func ValidateAppDescriptor(a AppDescriptor, index *bricksindex.BricksIndex) error {
	return validatebricks(a, index)
}

func validatebricks(a AppDescriptor, index *bricksindex.BricksIndex) error {
	for _, appBrick := range a.Bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			return fmt.Errorf("brick %q not found", appBrick.ID)
		}

		// check the bricks variables inside the app.yaml are valid given a brick definition
		for appBrickName := range appBrick.Variables {
			_, exist := indexBrick.GetVariable(appBrickName)
			if !exist {
				return fmt.Errorf("variable %q does not exist on brick %q", appBrickName, indexBrick.ID)
			}
		}

		// check all required variables has a value
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

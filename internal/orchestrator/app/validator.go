package app

import (
	"errors"
	"fmt"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

// ValidateBricks checks that all bricks referenced in the given AppDescriptor exist in the provided BricksIndex,
// It collects and returns all validatio errors as a single joined error, allowing the caller to see all issues at once rather than stopping at the first error.
// If the index or the app is nil, validation is skipped and nil is returned.
func ValidateBricks(a AppDescriptor, index *bricksindex.BricksIndex) error {
	if index == nil {
		return nil
	}

	var allErrors error

	for _, appBrick := range a.Bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			allErrors = errors.Join(allErrors, fmt.Errorf("brick %q not found", appBrick.ID))
			continue // Skip further validation for this brick since it doesn't exist
		}

		for appBrickName := range appBrick.Variables {
			_, exist := indexBrick.GetVariable(appBrickName)
			if !exist {
				allErrors = errors.Join(allErrors, fmt.Errorf("variable %q does not exist on brick %q", appBrickName, indexBrick.ID))
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

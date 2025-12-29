package app

import (
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

// ValidateBricks checks that all bricks referenced in the given AppDescriptor exist in the provided BricksIndex,
// It collects and returns all validation errors as a single joined error, allowing the caller to see all issues at once rather than stopping at the first error.
func ValidateBricks(a AppDescriptor, index *bricksindex.BricksIndex) error {
	if index == nil {
		return fmt.Errorf("bricks index cannot be nil")
	}

	var allErrors error

	for _, appBrick := range a.Bricks {
		indexBrick, found := index.FindBrickByID(appBrick.ID)
		if !found {
			allErrors = errors.Join(allErrors, fmt.Errorf("brick %q not found", appBrick.ID))
			continue // Skip further validation for this brick since it doesn't exist
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

func ValidateStructure(filePaths []string) error {
	var (
		hasAppYaml    bool
		hasMainPy     bool
		hasSketchDir  bool
		hasSketchIno  bool
		hasSketchYaml bool
	)

	for _, p := range filePaths {
		p = strings.TrimSpace(p)

		if p == "app.yaml" {
			hasAppYaml = true
		}

		if p == "python/main.py" {
			hasMainPy = true
		}

		if strings.HasPrefix(p, "sketch/") {
			hasSketchDir = true
			base := path.Base(p)
			if base == "sketch.ino" {
				hasSketchIno = true
			}
			if base == "sketch.yaml" {
				hasSketchYaml = true
			}
		}
	}

	var errs error

	if !hasAppYaml {
		errs = errors.Join(errs, errors.New("missing required file 'app.yaml'"))
	}
	if !hasMainPy {
		errs = errors.Join(errs, errors.New("missing required file 'python/main.py'"))
	}
	if hasSketchDir {
		if !hasSketchIno {
			errs = errors.Join(errs, errors.New("sketch directory present but 'sketch/sketch.ino' is missing"))
		}
		if !hasSketchYaml {
			errs = errors.Join(errs, errors.New("sketch directory present but 'sketch/sketch.yaml' is missing"))
		}
	}

	return errs
}

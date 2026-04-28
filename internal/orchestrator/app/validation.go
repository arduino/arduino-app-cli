package app

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func ValidateApp(appPath *paths.Path) error {
	if !IsValidFolderName(appPath.Base()) {
		return fmt.Errorf("root folder name %q is not valid: use only alphanumeric, underscores, dashes and spaces", appPath.Base())
	}
	return ValidateAppContent(appPath)
}

func ValidateAppContent(appPath *paths.Path) error {
	descriptorFile := appPath.Join("app.yaml")
	if _, err := validateAndParseDescriptor(descriptorFile); err != nil {
		return err
	}

	sketchPath := appPath.Join("sketch")
	if err := isValidSketchFolder(sketchPath); err != nil {
		return err
	}

	if !appPath.Join("python", "main.py").Exist() {
		return errors.New("main python file missing from app")
	}

	return nil
}

func IsValidFolderName(s string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9_ -]*$`, s)
	return matched
}

func validateAndParseDescriptor(descriptorFile *paths.Path) (AppDescriptor, error) {
	if !descriptorFile.Exist() {
		return AppDescriptor{}, errors.New("descriptor app.yaml file missing from app")
	}
	appDescriptor, err := ParseDescriptorFile(descriptorFile)
	if err != nil {
		return AppDescriptor{}, fmt.Errorf("error loading app descriptor file: %w", err)
	}
	return appDescriptor, nil
}

func isValidSketchFolder(sketchDir *paths.Path) error {
	if sketchDir == nil {
		return nil
	}

	sketchIno := sketchDir.Join("sketch.ino")
	sketchYaml := sketchDir.Join("sketch.yaml")

	if sketchIno.Exist() || sketchYaml.Exist() {
		if !sketchIno.Exist() || !sketchYaml.Exist() {
			return errors.New("sketch folder is incomplete: both sketch.ino and sketch.yaml are required")
		}
	}
	return nil
}

func isValid(brick bricksindex.Brick) error {
	if brick.ID == "" {
		return errors.New("brick ID is required")
	}
	// TODO: add other validation
	return nil
}

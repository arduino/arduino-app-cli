package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
)

// ArduinoApp holds all the files composing an app
type ArduinoApp struct {
	Name           string
	MainPythonFile *paths.Path
	MainSketchPath *paths.Path
	FullPath       *paths.Path // FullPath is the path to the App folder
	Descriptor     AppDescriptor
}

// Load creates an App instance by reading all the files composing an app and grouping them
// by file type.
func Load(appPath string) (ArduinoApp, error) {
	path := paths.New(appPath)
	if path == nil {
		return ArduinoApp{}, errors.New("empty app path")
	}

	exist, err := path.IsDirCheck()
	if err != nil {
		return ArduinoApp{}, fmt.Errorf("app path is not valid: %w", err)
	}
	if !exist {
		return ArduinoApp{}, fmt.Errorf("no such file or directory: %s", path)
	}
	path, err = path.Abs()
	if err != nil {
		return ArduinoApp{}, fmt.Errorf("cannot get absolute path for app: %w", err)
	}

	app := ArduinoApp{
		FullPath:   path,
		Descriptor: AppDescriptor{},
	}

	if descriptorFile := app.GetDescriptorPath(); descriptorFile.Exist() {
		desc, err := ParseDescriptorFile(descriptorFile)
		if err != nil {
			return ArduinoApp{}, fmt.Errorf("error loading app descriptor file: %w", err)
		}
		app.Descriptor = desc
		app.Name = desc.Name
	} else {
		return ArduinoApp{}, errors.New("descriptor app.yaml file missing from app")
	}

	if path.Join("python", "main.py").Exist() {
		app.MainPythonFile = path.Join("python", "main.py")
	}

	if path.Join("sketch", "sketch.ino").Exist() {
		// TODO: check sketch casing?
		app.MainSketchPath = path.Join("sketch")
	}

	if app.MainPythonFile == nil && app.MainSketchPath == nil {
		return ArduinoApp{}, errors.New("main python file and sketch file missing from app")
	}

	// Ensure the .cache folder is created
	if err := app.ProvisioningStateDir().MkdirAll(); err != nil {
		return ArduinoApp{}, errors.New("cannot create .cache directory for app provisioning state")
	}

	return app, nil
}

// GetDescriptorPath returns the path to the app descriptor file (app.yaml or app.yml)
func (a *ArduinoApp) GetDescriptorPath() *paths.Path {
	descriptorFile := a.FullPath.Join("app.yaml")
	if !descriptorFile.Exist() {
		alternateDescriptorFile := a.FullPath.Join("app.yml")
		if alternateDescriptorFile.Exist() {
			return alternateDescriptorFile
		}
	}
	return descriptorFile
}

var ErrInvalidApp = fmt.Errorf("invalid app")

func (a *ArduinoApp) Save() error {
	if err := a.Descriptor.IsValid(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidApp, err)
	}
	if err := a.writeApp(); err != nil {
		return err
	}
	return nil
}

func (a *ArduinoApp) writeApp() error {
	descriptorPath := a.GetDescriptorPath()
	if descriptorPath == nil {
		return errors.New("app descriptor file path is not set")
	}

	out, err := yaml.Marshal(a.Descriptor)
	if err != nil {
		return fmt.Errorf("cannot marshal app descriptor: %w", err)
	}

	if err := fatomic.WriteFile(descriptorPath.String(), out, os.FileMode(0644)); err != nil {
		return fmt.Errorf("cannot write app descriptor file: %w", err)
	}
	return nil
}

func (a *ArduinoApp) SketchBuildPath() *paths.Path {
	return a.FullPath.Join(".cache", "sketch")
}

func (a *ArduinoApp) ProvisioningStateDir() *paths.Path {
	return a.FullPath.Join(".cache")
}

func (a *ArduinoApp) AppComposeFilePath() *paths.Path {
	return a.ProvisioningStateDir().Join("app-compose.yaml")
}

func (a *ArduinoApp) AppComposeOverrideFilePath() *paths.Path {
	return a.ProvisioningStateDir().Join("app-compose-overrides.yaml")
}

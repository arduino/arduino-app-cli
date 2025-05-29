package parser

import (
	"errors"
	"fmt"
	"os"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"github.com/google/renameio"
)

// App holds all the files composing an app
type App struct {
	Name           string
	MainPythonFile *paths.Path
	MainSketchFile *paths.Path
	FullPath       *paths.Path // FullPath is the path to the App folder
	Descriptor     AppDescriptor
}

// Load creates an App instance by reading all the files composing an app and grouping them
// by file type.
func Load(appPath string) (App, error) {
	path := paths.New(appPath)
	if path == nil {
		return App{}, errors.New("empty app path")
	}

	path = path.Canonical()
	if exist, err := path.IsDirCheck(); err != nil {
		return App{}, fmt.Errorf("app path is not valid: %w", err)
	} else if !exist {
		return App{}, fmt.Errorf("no such file or directory: %s", path)
	}

	app := App{
		FullPath:   path,
		Descriptor: AppDescriptor{},
	}

	if descriptorFile := app.GetDescriptorPath(); descriptorFile.Exist() {
		desc, err := ParseDescriptorFile(descriptorFile)
		if err != nil {
			return App{}, fmt.Errorf("error loading app descriptor file: %w", err)
		}
		app.Descriptor = desc
		app.Name = desc.Name
	} else {
		return App{}, errors.New("descriptor app.yaml file missing from app")
	}

	if path.Join("python", "main.py").Exist() {
		app.MainPythonFile = path.Join("python", "main.py")
	}

	if path.Join("sketch", "sketch.ino").Exist() {
		// TODO: check sketch casing?
		app.MainSketchFile = path.Join("sketch", "sketch.ino")
	}

	if app.MainPythonFile == nil && app.MainSketchFile == nil {
		return App{}, errors.New("main python file and sketch file missing from app")
	}

	return app, nil
}

// GetDescriptorPath returns the path to the app descriptor file (app.yaml or app.yml)
func (a *App) GetDescriptorPath() *paths.Path {
	descriptorFile := a.FullPath.Join("app.yaml")
	if !descriptorFile.Exist() {
		alternateDescriptorFile := a.FullPath.Join("app.yml")
		if alternateDescriptorFile.Exist() {
			return alternateDescriptorFile
		}
	}
	return descriptorFile
}

func (a *App) Save() error {
	descriptorPath := a.GetDescriptorPath()
	if descriptorPath == nil {
		return errors.New("app descriptor file path is not set")
	}

	out, err := yaml.Marshal(a.Descriptor)
	if err != nil {
		return fmt.Errorf("cannot marshal app descriptor: %w", err)
	}

	if err := renameio.WriteFile(descriptorPath.String(), out, os.FileMode(0644)); err != nil {
		return fmt.Errorf("cannot write app descriptor file: %w", err)
	}
	return nil
}

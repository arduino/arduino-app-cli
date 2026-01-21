package aimodel

import (
	"errors"
	"fmt"
	"os"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
)

type CustomAiModel struct {
	FullPath        *paths.Path // Is the path to the model folder
	ModelDescriptor ModelDescriptor
}

func Load(path *paths.Path) (CustomAiModel, error) {
	if path == nil {
		return CustomAiModel{}, errors.New("empty model path")
	}

	exist, err := path.IsDirCheck()
	if err != nil {
		return CustomAiModel{}, fmt.Errorf("model path is not valid: %w", err)
	}
	if !exist {
		return CustomAiModel{}, fmt.Errorf("model path must be a directory: %s", path)
	}
	modelPath, err := path.Abs()
	if err != nil {
		return CustomAiModel{}, fmt.Errorf("cannot get absolute path for model: %w", err)
	}

	m := CustomAiModel{FullPath: modelPath}

	if descriptorFile := m.GetDescriptorPath(); descriptorFile.Exist() {
		desc, err := ParseModelDescriptorFile(descriptorFile)
		if err != nil {
			return CustomAiModel{}, fmt.Errorf("error loading model descriptor file: %w", err)
		}
		m.ModelDescriptor = desc
	} else {
		return CustomAiModel{}, errors.New("descriptor model.yaml file missing from app")
	}

	return m, nil
}

func Write(dir *paths.Path, descr ModelDescriptor) error {
	if err := dir.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	m := CustomAiModel{
		FullPath:        dir,
		ModelDescriptor: descr,
	}

	err := m.Save()
	if err != nil {
		return fmt.Errorf("failed to write model: %w", err)
	}

	return nil
}

func (a *CustomAiModel) GetDescriptorPath() *paths.Path {
	descriptorFile := a.FullPath.Join("model.yaml")
	if !descriptorFile.Exist() {
		alternateDescriptorFile := a.FullPath.Join("model.yml")
		if alternateDescriptorFile.Exist() {
			return alternateDescriptorFile
		}
	}
	return descriptorFile
}

var ErrInvalidModel = fmt.Errorf("invalid model")

func (a *CustomAiModel) Save() error {
	if err := a.ModelDescriptor.IsValid(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidModel, err)
	}
	if err := a.write(); err != nil {
		return err
	}
	return nil
}

func (a *CustomAiModel) write() error {
	descriptorPath := a.GetDescriptorPath()
	if descriptorPath == nil {
		return errors.New("model descriptor file path is not set")
	}

	out, err := yaml.Marshal(a.ModelDescriptor)
	if err != nil {
		return fmt.Errorf("cannot marshal model descriptor: %w", err)
	}

	if err := fatomic.WriteFile(descriptorPath.String(), out, os.FileMode(0644)); err != nil {
		return fmt.Errorf("cannot write model descriptor file: %w", err)
	}
	return nil
}

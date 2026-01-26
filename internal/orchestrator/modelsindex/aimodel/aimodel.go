package aimodel

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
)

type CustomAiModel struct {
	FullPath        *paths.Path // Is the path to the folder containing the model and the descriptor file
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

// Store creates a model directory with its descriptor file and optionally a blob file.
// If blobReader is provided, blobFilename specifies the name (defaults to "model.blob").
// The blob is written atomically with a size limit.
func Store(dir *paths.Path, descr ModelDescriptor, modelFileReader io.Reader, modelFilename string) error {
	if modelFileReader != nil && modelFilename == "" {
		return fmt.Errorf("model filename must be provided when model reader is not nil")
	}
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

	if modelFileReader != nil {
		destBlobPath := dir.Join(filepath.Base(modelFilename))
		f, err := os.Create(destBlobPath.String())
		if err != nil {
			return fmt.Errorf("failed to create model file: %w", err)
		}
		defer f.Close()
		if _, err := io.Copy(f, modelFileReader); err != nil {
			return fmt.Errorf("failed to write model file : %w", err)
		}
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

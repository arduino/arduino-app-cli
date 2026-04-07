// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package custommodel

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

type AiModel struct {
	FullPath        *paths.Path // Is the path to the folder containing the model and the descriptor file
	ModelDescriptor ModelDescriptor
}

func Load(path *paths.Path) (AiModel, error) {
	if path == nil {
		return AiModel{}, errors.New("empty model folder path")
	}

	exist, err := path.IsDirCheck()
	if err != nil {
		return AiModel{}, fmt.Errorf("model folder path is not valid: %w", err)
	}
	if !exist {
		return AiModel{}, fmt.Errorf("model folder path must be a directory: %s", path)
	}
	modelFolderPath, err := path.Abs()
	if err != nil {
		return AiModel{}, fmt.Errorf("cannot get absolute path for model: %w", err)
	}

	m := AiModel{FullPath: modelFolderPath}

	if descriptorFile := m.GetDescriptorPath(); descriptorFile.Exist() {
		desc, err := ParseModelDescriptorFile(descriptorFile)
		if err != nil {
			return AiModel{}, fmt.Errorf("error loading model descriptor file: %w", err)
		}
		m.ModelDescriptor = desc
	} else {
		return AiModel{}, errors.New("descriptor model.yaml file missing from app")
	}

	return m, nil
}

func Store(dir *paths.Path, descr ModelDescriptor, modelFileReader io.ReadCloser, modelFilename string) (AiModel, error) {
	if modelFileReader == nil {
		return AiModel{}, fmt.Errorf("model file reader cannot be nil")
	}
	if modelFilename == "" {
		return AiModel{}, fmt.Errorf("model filename must be provided when model reader is not nil")
	}

	if err := dir.MkdirAll(); err != nil {
		return AiModel{}, fmt.Errorf("failed to create model directory: %w", err)
	}

	defer modelFileReader.Close()

	destBlobPath := dir.Join(filepath.Base(modelFilename))
	f, err := os.Create(destBlobPath.String())
	if err != nil {
		return AiModel{}, fmt.Errorf("failed to create model file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, modelFileReader); err != nil {
		_ = destBlobPath.Remove()
		return AiModel{}, fmt.Errorf("failed to write model file: %w", err)
	}

	m := AiModel{
		FullPath:        dir,
		ModelDescriptor: descr,
	}

	err = m.writeDescriptorFile()
	if err != nil {
		return AiModel{}, fmt.Errorf("failed to write model: %w", err)
	}
	return m, nil
}

func (a *AiModel) GetDescriptorPath() *paths.Path {
	return a.FullPath.Join("model.yaml")
}

func (a *AiModel) writeDescriptorFile() error {
	if !a.ModelDescriptor.IsValid() {
		// TODO: provide more details about the invalidity
		return errors.New("invalid model descriptor")
	}
	descriptorPath := a.GetDescriptorPath()
	if descriptorPath == nil {
		return errors.New("model descriptor file path is not set")
	}

	out, err := yaml.Marshal(a.ModelDescriptor)
	if err != nil {
		return fmt.Errorf("cannot marshal model descriptor: %w", err)
	}

	if err := fatomic.WriteFile(descriptorPath.String(), out, os.FileMode(0644)); err != nil {
		_ = descriptorPath.Remove()
		return fmt.Errorf("failed to write model descriptorfile: %w", err)
	}
	return nil
}

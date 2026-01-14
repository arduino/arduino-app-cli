package custommodels

import (
	"io/fs"
	"path/filepath"

	"github.com/arduino/go-paths-helper"
	"gopkg.in/yaml.v3"
)

var eiCategoryToArduinoBrick = map[string]string{
	"Images": "object-detection",
}

type EdgeImpulseModel struct {
	Id          string `yaml:"-"`
	ProjectId   int    `yaml:"project-id"`
	ImpulseID   int    `yaml:"impulse-id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Category    string `yaml:"category"`
	Path        string `yaml:"-"`
}

func List(eiModelsPath *paths.Path) ([]EdgeImpulseModel, error) {
	models := make([]EdgeImpulseModel, 0)

	err := filepath.WalkDir(eiModelsPath.String(), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		base := filepath.Base(path)
		if base != "metadata.yml" && base != "metadata.yaml" {
			return nil
		}

		f, err := paths.New(path).Open()
		if err != nil {
			return err
		}
		defer f.Close()

		var mf EdgeImpulseModel
		if err := yaml.NewDecoder(f).Decode(&mf); err != nil {
			return err
		}
		mf.Path = filepath.Dir(path)

		models = append(models, mf)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return models, nil
}

func GetModelsByBrick(eiModelsPath *paths.Path, brickId string) ([]EdgeImpulseModel, error) {
	models, err := List(eiModelsPath)
	if err != nil {
		return nil, err
	}

	var matchedModels []EdgeImpulseModel
	for _, model := range models {
		if eiCategoryToArduinoBrick[model.Category] == brickId {
			matchedModels = append(matchedModels, model)
		}
	}

	return matchedModels, nil
}

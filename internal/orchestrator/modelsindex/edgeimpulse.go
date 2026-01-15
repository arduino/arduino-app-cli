package modelsindex

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
)

// map Edge Impulse categories to Arduino bricks
var eiCategoryToArduinoBrick = map[string]string{
	"Images": "object-detection",
	// TODO: define the mapping missing
}

func LoadEdgeImpulseModels(dir *paths.Path) ([]AIModel, error) {
	type modelDescriptor struct {
		ProjectId   int    `yaml:"project-id"`
		ImpulseID   int    `yaml:"impulse-id"`
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Category    string `yaml:"category"`
		Path        string `yaml:"-"`
	}
	var models []AIModel
	err := filepath.WalkDir(dir.String(), func(path string, d fs.DirEntry, walkErr error) error {
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

		var mf modelDescriptor
		if err := yaml.NewDecoder(f).Decode(&mf); err != nil {
			return err
		}
		mf.Path = filepath.Dir(path)

		models = append(models, AIModel{
			ID:                fmt.Sprintf("%d-%d", mf.ProjectId, mf.ImpulseID), // TODO: generation of ID
			Source:            "edgeimpulse",
			Name:              mf.Name,
			ModuleDescription: mf.Description,
			Runner:            "bricks",
			Bricks:            []string{eiCategoryToArduinoBrick[mf.Category]},
			Metadata: map[string]string{
				"project-id": fmt.Sprintf("%d", mf.ProjectId),
				"impulse-id": fmt.Sprintf("%d", mf.ImpulseID),
			},
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return models, nil
}

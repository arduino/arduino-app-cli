package modelsindex

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
)

type arduinoBrickConfig struct {
	brickID               string
	configurationVariable string
}

// map Edge Impulse categories to Arduino bricks
var eiCategoryToArduinoBrick = map[string][]arduinoBrickConfig{
	"Images": {
		{
			brickID:               "object-detection",
			configurationVariable: "EI_OBJ_DETECTION_MODEL",
		},
	},
	// TODO: define missing mapping
}

func LoadEdgeImpulseModels(dir *paths.Path) ([]AIModel, error) {
	if dir == nil {
		return []AIModel{}, nil
	}
	type modelDescriptor struct {
		ID          string `yaml:"id"`
		ProjectId   int    `yaml:"project-id"`
		ImpulseID   int    `yaml:"impulse-id"`
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Category    string `yaml:"category"`
		Path        string `yaml:"path"`
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
		var bricks []string
		var modelConfig = make(map[string]string)
		for _, b := range eiCategoryToArduinoBrick[mf.Category] {
			bricks = append(bricks, b.brickID)
			// FIXME: based on the name of the config different value myust be resolved
			modelConfig[b.configurationVariable] = paths.New(path).Parent().Join(mf.Path).String()
		}

		models = append(models, AIModel{
			ID:                mf.ID,
			Source:            "edgeimpulse",
			Name:              mf.Name,
			ModuleDescription: mf.Description,
			Runner:            "bricks",
			Metadata: map[string]string{
				"project-id": fmt.Sprintf("%d", mf.ProjectId),
				"impulse-id": fmt.Sprintf("%d", mf.ImpulseID),
			},
			Bricks:             bricks,
			ModelConfiguration: modelConfig,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return models, nil
}

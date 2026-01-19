package modelsindex

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

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
}

// TODO: define missing mapping
type modelDescriptor struct {
	ID          string    `yaml:"id"`
	ProjectID   int       `yaml:"project-id"`
	ImpulseID   int       `yaml:"impulse-id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Category    string    `yaml:"category"`
	Path        string    `yaml:"model_path"` //TODO do we need this?
	LastBuildAt time.Time `yaml:"lastBuildAt" json:"lastBuildAt"`
	BrickIDs    []string  `yaml:"brick_ids" json:"brick_ids"`
}

func InstallEIModel(ctx context.Context, eiClient *EIClient, EImodelPath paths.Path, EIprojectID int, EIimpulseID int) error {

	version, err := eiClient.GetDeployment(ctx, EIprojectID)
	if err != nil {
		return err
	}
	if version == nil {
		jobId, err := eiClient.Build(ctx, EIprojectID)
		if err != nil {
			return err
		}
		err = eiClient.WaitForBuildCompletion(ctx, EIprojectID, *jobId)
		if err != nil {
			return err
		}
	}

	err = eiClient.DownloadAndInstallModel(ctx, EImodelPath.String(), EIprojectID, EIimpulseID)
	if err != nil {
		return err
	}

	return nil
}

func EItoArduinoModel(EICategory string, Impulse *string) []string {
	switch EICategory {
	case "Object detection":
		return []string{"arduino:object_detection", "arduino:video_object_detection"}
	case "Images":
		if Impulse != nil && *Impulse == "keras-visual-anomaly" {
			return []string{"arduino:visual_anomaly_detection"}
		}
		return []string{"arduino:image_classification", "arduino:video_image_classification"}
	case "Audio":
		return []string{"arduino:audio_classification"}
	case "Keyword spotting":
		return []string{"arduino:audio_classification", "arduino:keyword_spotting"}
	case "Accelerometer":
		return []string{"arduino:gesture_recognition", "arduino:anomaly_detection"}
	default:
		panic("Unknown category")
	}
}

func SaveEIModel(ctx context.Context, eiClient *EIClient, modelPath paths.Path, projectID int, impulseID int) (*AIModel, error) {

	project, err := eiClient.GetProjectInfo(ctx, projectID)
	if err != nil {
		return nil, err
	}

	metadataFile := modelDescriptor{
		ID:          fmt.Sprintf("%d-%d", projectID, impulseID),
		ProjectID:   projectID,
		ImpulseID:   impulseID,
		Name:        project.Name,
		Description: project.Description,
		Category:    project.Category,
		LastBuildAt: project.LastModified,
		BrickIDs:    EItoArduinoModel(project.Category, nil),
	}

	data, err := yaml.Marshal(metadataFile)
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(modelPath.String()+"/metadata.yaml", data, 0o644)
	if err != nil {
		return nil, err
	}

	return &AIModel{
		ID:                fmt.Sprintf("%d-%d", projectID, impulseID),
		Source:            "edgeimpulse",
		Name:              project.Name,
		ModuleDescription: project.Description,
		Runner:            "bricks",
		Bricks:            EItoArduinoModel(project.Category, nil),
		Metadata: map[string]string{
			"project-id": fmt.Sprintf("%d", projectID),
			"impulse-id": fmt.Sprintf("%d", impulseID),
		},
	}, nil

}

func LoadEdgeImpulseModels(dir *paths.Path) ([]AIModel, error) {
	if dir == nil {
		return []AIModel{}, nil
	}
	type modelDescriptor struct {
		ID          string `yaml:"id"`
		ProjectID   int    `yaml:"project-id"`
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
				"project-id": fmt.Sprintf("%d", mf.ProjectID),
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

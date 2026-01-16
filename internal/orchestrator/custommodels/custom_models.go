package custommodels

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
)

type AIModel struct {
	ID                 string            `yaml:"-"`
	Source             string            `yaml:"source"`
	Name               string            `yaml:"name"`
	ModuleDescription  string            `yaml:"description"`
	Runner             string            `yaml:"runner"`
	Bricks             []string          `yaml:"bricks,omitempty"`
	ModelLabels        []string          `yaml:"model_labels,omitempty"`
	Metadata           map[string]string `yaml:"metadata,omitempty"`
	ModelConfiguration map[string]string `yaml:"model_configuration,omitempty"`
}

// Download and install a custom model identified by projectID and ImpulseID
// Create the matadatafile using the info from the model

type CustomModel struct {
	EIClient *EIClient
}

type YamlFile struct {
	ID          string    `yaml:"id" json:"id"`
	ProjectID   string    `yaml:"project-id" json:"project-id"`
	ImpulseID   string    `yaml:"impulse-id" json:"impulse-id"`
	Name        string    `yaml:"name" json:"name"`
	Description string    `yaml:"description" json:"description"`
	Category    string    `yaml:"category" json:"category"`
	LastBuildAt time.Time `yaml:"lastBuildAt" json:"lastBuildAt"`
	BrickIDs    []string  `yaml:"brick_ids" json:"brick_ids"`
}

func NewCustomModel(apiKey string, ApiUrl string, modelRoot paths.Path) *CustomModel {
	return &CustomModel{
		EIClient: NewEIClient(apiKey, ApiUrl, modelRoot),
	}
}

func (cm *CustomModel) Install(ctx context.Context, projectID string, ImpulseID string) error {

	version, err := cm.EIClient.GetDeployment(ctx, projectID)
	if err != nil {
		return err
	}
	if version == nil {
		jobId, err := cm.EIClient.Build(ctx, projectID)
		if err != nil {
			return err
		}
		err = cm.EIClient.WaitForBuildCompletion(ctx, projectID, *jobId)
		if err != nil {
			return err
		}
	}

	err = cm.EIClient.DownloadAndInstallModel(ctx, cm.EIClient.ModelRoot.String(), projectID, ImpulseID)
	if err != nil {
		return err
	}

	err = cm.createMetadataFile(ctx, cm.EIClient.ModelRoot, projectID, ImpulseID)
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

func (cm *CustomModel) createMetadataFile(ctx context.Context, modelPath paths.Path, projectID string, ImpulseID string) error {

	project, err := cm.EIClient.GetProjectInfo(ctx, projectID)
	if err != nil {
		return err
	}

	metadataFile := YamlFile{
		ID:          fmt.Sprintf("%d", project.ID),
		ProjectID:   projectID,
		ImpulseID:   ImpulseID,
		Name:        project.Name,
		Description: project.Description,
		Category:    project.Category,
		LastBuildAt: project.LastModified,
		BrickIDs:    EItoArduinoModel(project.Category, nil),
	}

	data, err := yaml.Marshal(metadataFile)
	if err != nil {
		return err
	}

	err = os.WriteFile(modelPath.String()+"/metadata.yaml", data, 0o644)
	if err != nil {
		return err
	}

	return nil

}

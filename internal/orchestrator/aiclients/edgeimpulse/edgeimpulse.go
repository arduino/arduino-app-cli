package edgeimpulse

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	yaml "github.com/goccy/go-yaml"
)

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

func InstallEIModel(ctx context.Context, eiClient *EIClient, EImodelPath string, EIprojectID int, EIimpulseID int, modelType string, engine string) error {

	modelTypeParam := ModelTypeParameter(modelType)
	engineParam := ModelEngineParameter(engine)

	version, err := eiClient.GetDeployment(ctx, EIprojectID, modelTypeParam, engineParam)
	if err != nil {
		return err
	}
	if version == nil {
		jobId, err := eiClient.Build(ctx, EIprojectID, modelTypeParam, engineParam)
		if err != nil {
			return err
		}
		err = eiClient.WaitForBuildCompletion(ctx, EIprojectID, *jobId)
		if err != nil {
			return err
		}
	}

	err = eiClient.DownloadAndInstallModel(ctx, EImodelPath, EIprojectID, EIimpulseID, modelTypeParam, engineParam)
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

func SaveEIModel(ctx context.Context, eiClient *EIClient, modelPath string, projectID int, impulseID int) (*modelsindex.AIModel, error) {

	project, err := eiClient.GetProjectInfo(ctx, projectID, impulseID)
	if err != nil {
		return nil, err
	}

	metadataFile := modelDescriptor{
		ID:          fmt.Sprintf("%d-%d", projectID, impulseID),
		ProjectID:   projectID,
		ImpulseID:   impulseID,
		Name:        project.Name,
		Description: project.Description,
		Category:    string(*project.Category),
		LastBuildAt: *project.LastModified, //TODO lastModified could not be accurate
		BrickIDs:    EItoArduinoModel(string(*project.Category), nil),
	}

	data, err := yaml.Marshal(metadataFile)
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(filepath.Join(modelPath, "metadata.yaml"), data, 0o644)
	if err != nil {
		return nil, err
	}

	return &modelsindex.AIModel{
		ID:                fmt.Sprintf("%d-%d", projectID, impulseID),
		Source:            "edgeimpulse",
		Name:              project.Name,
		ModuleDescription: project.Description,
		Runner:            "bricks",
		Bricks:            EItoArduinoModel(string(*project.Category), nil),
		Metadata: map[string]string{
			"project-id": fmt.Sprintf("%d", projectID),
			"impulse-id": fmt.Sprintf("%d", impulseID),
		},
	}, nil

}

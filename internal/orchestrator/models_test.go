package orchestrator

import (
	"context"
	"log"
	"os"
	"testing"

	paths "github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"

	"github.com/arduino/arduino-app-cli/internal/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

// TODO should be mocked and add a e2e test with an Arduino organization
func TestInstallEIModel(t *testing.T) {
	t.Skip("skipping EI model installation test, uncomment to run it")

	modelsDir := paths.New(os.Getenv("ARDUINO_CUSTOM_MODEL_PATH"))
	ApiKey := os.Getenv("ARDUINO_EI_API_KEY")
	ApiUrl := os.Getenv("ARDUINO_EI_API_URL")

	if err := os.MkdirAll(modelsDir.String(), 0o755); err != nil {
		log.Fatalf("failed to create directory %s: %v", modelsDir, err)
	}
	eiClient := edgeimpulse.NewEIClient(ApiKey, ApiUrl, "v1")
	ProjectID := 876194
	ImpulseID := 1

	// TODO: pass a brick index
	err := InstallEIModel(context.TODO(), nil, eiClient, modelsDir, ProjectID, ImpulseID, "int8", "tflite")
	if err != nil {
		log.Fatalf("failed to install EI model: %v", err)
	}

	models, err := modelsindex.Load(nil, modelsDir)
	if err != nil {
		log.Fatalf("failed to load EI models: %v", err)
	}

	assert.Len(t, models.GetModels(), 1, "expected one model to be installed")

	// found := models.GetModels()[0]
	// TODO: check the correct name
	// assert.Equal(t, model.Name, found.Name, "model name does not match")
	// assert.Equal(t, model.ModuleDescription, found.ModuleDescription, "model description does not match")
	// assert.Equal(t, model.Runner, found.Runner, "model runner does not match")
	// assert.Equal(t, model.Bricks, found.Bricks, "model bricks do not match")
	// assert.Equal(t, model.Metadata, found.Metadata, "model metadata does not match")

	if err := os.RemoveAll(modelsDir.String()); err != nil {
		log.Fatalf("failed to delete %s: %v", modelsDir, err)
	}
}

package custommodels

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
)

func TestInstall(t *testing.T) {

	customEIModelsDir := paths.New(os.Getenv("ARDUINO_CUSTOM_MODEL_PATH"))
	ApiKey := os.Getenv("ARDUINO_EI_API_KEY")
	ApiUrl := os.Getenv("ARDUINO_EI_API_URL")

	if err := os.MkdirAll(customEIModelsDir.String(), 0o755); err != nil {
		log.Fatalf("failed to create directory %s: %v", customEIModelsDir, err)
	}

	cm := NewCustomModel(ApiKey, ApiUrl, *customEIModelsDir)

	ProjectID := "876194"
	ImpulseID := "1"

	err := cm.Install(context.TODO(), ProjectID, ImpulseID)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if err := os.RemoveAll(customEIModelsDir.Parent().String()); err != nil {
		log.Fatalf("failed to delete %s: %v", customEIModelsDir, err)
	}
}

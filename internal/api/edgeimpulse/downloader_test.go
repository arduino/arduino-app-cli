package edgeimpulse

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/custommodel"
)

/*
TODO
- project has no impulse => error on building requiring to crate an ipmulse first
- project has a wrong target  (no the ardiuno-uno-q) ?
- if no imulse id is provided, get the latest one from the project in th app-yaml
- building a project with not existing impulsse => "Failed to run impulse Impulse is null"
*/
func TestCheckOrigin(t *testing.T) {
	httpClient, err := NewClientWithResponses("https://studio.edgeimpulse.com/v1", WithRequestEditorFn(func(ctx context.Context, ri *http.Request) error {
		ri.Header.Add("x-api-key", "ei_1faecd1e799db6a9c93629e453fbc25d2535603b95de37c11380a03bf01e713b")
		ri.Header.Set("Content-Type", "application/json")
		return nil
	}))
	if err != nil {
		t.Fatalf("unable to create Edge Impulse client: %v", err)
	}

	projectID := 889978
	modelType := ModelTypeParameter("float32")
	impulseID := 1

	downloader := NewProjectDownloader(httpClient, projectID)

	customModelDescriptor := custommodel.ModelDescriptor{
		ID:          "test",
		Name:        "test",
		Description: "test description",
		Metadata: map[string]string{
			"source":        "edgeimpulse",
			"ei-project-id": fmt.Sprintf("%d", downloader.ProjectID),
			"ei-impulse-id": fmt.Sprintf("%d", impulseID),
			"ei-model-type": string(modelType),
		},
		// Bricks: buildBrickConfigForEIModel(bricksIndex, project.Category, edgeModelsDir, blobModelsDir),
	}

	reader, err := downloader.DownloadDeployment(context.Background(), &impulseID, modelType)
	if err != nil {
		t.Fatalf("unable to download deployment: %v", err)
	}
	defer reader.Close()
	err = paths.New("models").MkdirAll()
	if err != nil {
		t.Fatalf("unable to create models dir: %v", err)
	}

	_, err = custommodel.Store(paths.New("models").Join(fmt.Sprintf("%d-%d-%s", projectID, impulseID, string(modelType))), customModelDescriptor, reader, "mio.eim")

	if err != nil {
		t.Fatalf("unable to store model: %v", err)
	}
}

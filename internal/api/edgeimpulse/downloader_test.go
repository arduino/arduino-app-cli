package edgeimpulse

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

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
	t.Skip("edge impulse local test only with api key")

	url := "https://studio.edgeimpulse.com/v1"
	k := os.Getenv("EDGE_IMPULSE_API_KEY")
	projectID := 889978
	modelType := ModelTypeParameter("float32")
	impulseID := 1

	downloader, err := NewProjectDownloader(url, k, projectID)
	require.NoError(t, err, "unable to create models dir")

	reader, err := downloader.DownloadDeployment(context.Background(), &impulseID, modelType)
	require.NoError(t, err, "unable to create models dir")

	modelDir := paths.New(t.TempDir()).Join(fmt.Sprintf("%d-%d-%s", projectID, impulseID, string(modelType)))
	err = modelDir.MkdirAll()
	require.NoError(t, err, "unable to create models dir")

	descri := custommodel.ModelDescriptor{
		ID:          "test",
		Name:        "test",
		Description: "test description",
		Metadata: map[string]string{
			"source":        "edgeimpulse",
			"ei-project-id": fmt.Sprintf("%d", downloader.ProjectID),
			"ei-impulse-id": fmt.Sprintf("%d", impulseID),
			"ei-model-type": string(modelType),
		},
	}
	_, err = custommodel.Store(modelDir, descri, reader, "model.eim")
	require.NoError(t, err, "unable to create models dir")
}

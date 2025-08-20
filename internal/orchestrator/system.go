package orchestrator

import (
	"context"
	"os"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/store"
)

// SystemInit pulls necessary Docker images.
func SystemInit(ctx context.Context, pythonImageTag string, staticStore *store.StaticStore) error {
	preInstallContainer := []string{
		"ghcr.io/bcmi-labs/arduino/appslab-python-apps-base:" + pythonImageTag,
	}
	additionalContainers, err := parseAllModelsRunnerImageTag(staticStore)
	if err != nil {
		return err
	}
	preInstallContainer = append(preInstallContainer, additionalContainers...)

	stdout, _, err := feedback.DirectStreams()
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		return nil
	}

	for _, container := range preInstallContainer {
		cmd, err := paths.NewProcess(nil, "docker", "pull", container)
		if err != nil {
			return err
		}
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)
		if err := cmd.RunWithinContext(ctx); err != nil {
			return err
		}
	}
	return nil
}

func parseAllModelsRunnerImageTag(staticStore *store.StaticStore) ([]string, error) {
	composePath := staticStore.GetComposeFolder()
	brickNamespace := "arduino"
	bricks, err := composePath.Join(brickNamespace).ReadDir()
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(bricks))
	for _, brick := range bricks {
		composeFile := composePath.Join(brickNamespace, brick.Base(), "brick_compose.yaml")
		content, err := composeFile.ReadFile()
		if err != nil {
			return nil, err
		}
		prj, err := loader.LoadWithContext(
			context.Background(),
			types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{{Content: content}},
				Environment: types.NewMapping(os.Environ()),
			},
			func(o *loader.Options) { o.SetProjectName("test", false) },
		)
		if err != nil {
			return nil, err
		}
		for _, v := range prj.Services {
			// Add only if the image comes from arduino
			if strings.HasPrefix(v.Image, "ghcr.io/bcmi-labs/arduino/") ||
				// TODO: add the correct ecr prefix as soon as we have it in production
				strings.HasPrefix(v.Image, "public.ecr.aws/") {
				result = append(result, v.Image)
			}
		}
	}

	return f.Uniq(result), nil
}

package main

import (
	"context"
	"os"
	"path"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/spf13/cobra"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/assets"
)

func newSystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "system",
		Hidden: true,
	}

	cmd.AddCommand(newDownloadImage())

	return cmd
}

func newDownloadImage() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "init",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return SystemInit(cmd.Context())
		},
	}

	return cmd
}

// SystemInit pulls necessary Docker images.
func SystemInit(ctx context.Context) error {
	preInstallContainer := []string{
		"ghcr.io/bcmi-labs/arduino/appslab-python-apps-base:" + usedPythonImageTag,
	}
	additionalContainers, err := parseAllModelsRunnerImageTag()
	if err != nil {
		return err
	}
	preInstallContainer = append(preInstallContainer, additionalContainers...)

	for _, container := range preInstallContainer {
		cmd, err := paths.NewProcess(nil, "docker", "pull", container)
		if err != nil {
			return err
		}
		cmd.RedirectStderrTo(os.Stdout)
		cmd.RedirectStdoutTo(os.Stdout)
		if err := cmd.RunWithinContext(ctx); err != nil {
			return err
		}
	}
	return nil
}

func parseAllModelsRunnerImageTag() ([]string, error) {
	versions, err := assets.FS.ReadDir("static")
	if err != nil {
		return nil, err
	}
	f.Assert(len(versions) == 1, "No models available in the assets directory")

	baseDir := path.Join("static", versions[0].Name(), "compose", "arduino")
	bricks, err := assets.FS.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(bricks))
	for _, brick := range bricks {
		composeFile := path.Join(baseDir, brick.Name(), "brick_compose.yaml")
		content, err := assets.FS.ReadFile(composeFile)
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

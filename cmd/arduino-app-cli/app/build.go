// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func newBuildCmd(cfg config.Configuration) *cobra.Command {
	var (
		target    string
		version   string
		notes     string
		output    string
		overwrite bool
	)

	cmd := &cobra.Command{
		Use:   "build app_path",
		Short: "Build an Arduino App into a release archive",
		Long: `Build an Arduino App into a release archive.

The release is the app frozen with all its dependencies: the python environment is
built and the compose files are resolved for the target board, so that installing
it generates nothing. The version defaults to a UTC timestamp and the target board
to the one running the build.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			appToBuild, err := Load(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}

			req := orchestrator.BuildReleaseRequest{
				Target:    target,
				Version:   version,
				Overwrite: overwrite,
			}
			if notes != "" {
				req.Notes = paths.New(notes)
				if req.Notes.NotExist() {
					feedback.Fatal(fmt.Sprintf("Notes file %q not found", notes), feedback.ErrBadArgument)
				}
			}
			if output != "" {
				req.Output = paths.New(output)
			}

			return buildHandler(cmd.Context(), cfg, appToBuild, req)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return completion.ApplicationNamesWithFilterFunc(cfg, func(apps orchestrator.AppInfo) bool {
				return !apps.Example
			})(cmd, args, toComplete)
		},
	}

	cmd.Flags().StringVar(&target, "target", "", fmt.Sprintf("Board the release is built for (%s). Defaults to the board running the build", strings.Join(platform.SupportedBoards(), ", ")))
	cmd.Flags().StringVar(&version, "version", "", "Release version. Defaults to a UTC timestamp")
	cmd.Flags().StringVar(&notes, "notes", "", "File with the release notes")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output archive, or the directory to write it in")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite the output archive if it exists")

	return cmd
}

func buildHandler(ctx context.Context, cfg config.Configuration, appToBuild app.ArduinoApp, req orchestrator.BuildReleaseRequest) error {
	// First: creating it is what fills the asset dir the indexes are read from.
	provisioner := servicelocator.GetProvisioner()

	targetPlatform, bricksIndex, servicesIndex, modelsIndex, err := targetIndexes(cfg, req.Target)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}

	out, _, getResult := feedback.OutputStreams()

	result, err := orchestrator.BuildRelease(
		ctx,
		servicelocator.GetDockerClient(),
		provisioner,
		modelsIndex,
		bricksIndex,
		servicesIndex,
		appToBuild,
		req,
		cfg,
		targetPlatform,
		func(message orchestrator.StreamMessage) {
			switch message.GetType() {
			case orchestrator.ProgressType:
				fmt.Fprintf(out, "Progress[%s]: %.0f%%\n", message.GetProgress().Name, message.GetProgress().Progress)
			case orchestrator.InfoType:
				fmt.Fprintln(out, "[INFO]", message.GetData())
			}
		},
	)
	if err != nil {
		feedback.Fatal(fmt.Sprintf("[ERROR] %s", err), feedback.ErrGeneric)
	}

	feedback.PrintResult(buildAppResult{
		BuildReleaseResult: result,
		Output:             getResult(),
	})
	return nil
}

// targetIndexes are the indexes of the board the release is built for: which bricks
// and services exist, and which compose variant they use, depend on it.
func targetIndexes(cfg config.Configuration, target string) (platform.Platform, *bricksindex.BricksIndex, *servicesindex.ServicesIndex, *modelsindex.ModelsIndex, error) {
	if target == "" {
		return servicelocator.GetPlatform(), servicelocator.GetBricksIndex(), servicelocator.GetServicesIndex(), servicelocator.GetModelsIndex(), nil
	}

	targetPlatform, ok := platform.ForBoard(target)
	if !ok {
		return platform.Platform{}, nil, nil, nil, fmt.Errorf("unknown target board %q: expected one of %s", target, strings.Join(platform.SupportedBoards(), ", "))
	}
	if targetPlatform.BoardName == servicelocator.GetPlatform().BoardName {
		return servicelocator.GetPlatform(), servicelocator.GetBricksIndex(), servicelocator.GetServicesIndex(), servicelocator.GetModelsIndex(), nil
	}

	bricksIndex, err := bricksindex.Load(targetPlatform, cfg.AssetDir())
	if err != nil {
		return platform.Platform{}, nil, nil, nil, fmt.Errorf("failed to load the bricks index of %s: %w", target, err)
	}
	servicesIndex, err := servicesindex.Load(targetPlatform, cfg.AssetDir().Join("services"))
	if err != nil {
		return platform.Platform{}, nil, nil, nil, fmt.Errorf("failed to load the services index of %s: %w", target, err)
	}
	modelsIndex, err := modelsindex.Load(targetPlatform, cfg.AssetDir(), cfg.ModelsDir(), cfg.CustomModelsDir(), servicelocator.GetDockerClient().Client(), cfg)
	if err != nil {
		return platform.Platform{}, nil, nil, nil, fmt.Errorf("failed to load the models index of %s: %w", target, err)
	}
	return targetPlatform, bricksIndex, servicesIndex, modelsIndex, nil
}

type buildAppResult struct {
	orchestrator.BuildReleaseResult
	Output *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r buildAppResult) String() string {
	return fmt.Sprintf("✓ Release %s %s built for %s in '%s'", r.Name, r.Version, r.Target, r.Archive)
}

func (r buildAppResult) Data() any {
	return r
}

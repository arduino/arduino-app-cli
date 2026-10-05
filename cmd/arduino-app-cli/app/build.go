// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/releasebuild"
)

func newBuildCmd(cfg config.Configuration) *cobra.Command {
	var (
		target       string
		releaseLabel string
		notes        string
		output       string
		includeData  bool
		overwrite    bool
		verbose      bool
	)

	cmd := &cobra.Command{
		Use:   "build app_path",
		Short: "Build an Arduino App into a release archive",
		Long: `Build an Arduino App into a release archive.

The release is the app frozen with all its dependencies: the python environment is
built and the compose files are resolved for the target board, so that installing
it generates nothing. The release is named after the build date, and the target
board defaults to the one running the build.`,
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
				Target:       target,
				ReleaseLabel: releaseLabel,
				IncludeData:  includeData,
				Verbose:      verbose,
			}
			if notes != "" {
				req.Notes = readReleaseNotes(notes)
			}
			var outputPath *paths.Path
			if output != "" {
				outputPath = paths.New(output)
			}

			return buildHandler(cmd.Context(), cfg, appToBuild, req, outputPath, overwrite)
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
	cmd.Flags().StringVar(&releaseLabel, "release-label", "", "Optional label to attach to the release, stored in its manifest")
	cmd.Flags().StringVar(&notes, "notes", "", "File with the release notes, or - to read them from the standard input")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output archive, which names the release folder as well, or the directory to write it in")
	cmd.Flags().BoolVar(&includeData, "include-data", false, "Include data directory in the archive")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite the output archive if it exists")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return cmd
}

func buildHandler(ctx context.Context, cfg config.Configuration, appToBuild app.ArduinoApp, req orchestrator.BuildReleaseRequest, output *paths.Path, overwrite bool) error {
	// First: creating it is what fills the asset dir the indexes are read from.
	provisioner := servicelocator.GetProvisioner()

	out, _, getResult := feedback.OutputStreams()

	var archivePath *paths.Path
	if output != nil && !output.IsDir() {
		resolved, err := releasebuild.ResolveArchivePath("", output, overwrite)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		}
		archivePath = resolved
		req.ReleaseName = strings.TrimSuffix(resolved.Base(), resolved.Ext())
	}

	result, reader, err := orchestrator.BuildRelease(
		ctx,
		servicelocator.GetDockerClient(),
		provisioner,
		appToBuild,
		req,
		cfg,
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
		if errors.Is(err, orchestrator.ErrBadRequest) {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		}
		feedback.Fatal(fmt.Sprintf("[ERROR] %s", err), feedback.ErrGeneric)
	}

	// A dir or nothing passed to the --output flag leaves the naming to the build, so the
	// archive path is known only now.
	if archivePath == nil {
		resolved, err := releasebuild.ResolveArchivePath(result.FileName, output, overwrite)
		if err != nil {
			_ = reader.Close()
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		}
		archivePath = resolved
	}

	if err := saveReleaseArchive(reader, archivePath); err != nil {
		feedback.Fatal(fmt.Sprintf("[ERROR] %s", err), feedback.ErrGeneric)
	}

	feedback.PrintResult(buildAppResult{
		BuildReleaseResult: result,
		Archive:            archivePath.String(),
		Output:             getResult(),
	})
	return nil
}

// saveReleaseArchive writes the release stream to the archive, closing the stream so the
// build staging dir goes with it, and removing a half written archive on failure.
func saveReleaseArchive(reader io.ReadCloser, archivePath *paths.Path) (err error) {
	defer func() {
		if closeErr := reader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			// Do not leave a half written archive behind.
			_ = archivePath.Remove()
		}
	}()

	return releasebuild.WriteArchive(reader, archivePath)
}

// readReleaseNotes is the note the release ships in its manifest: a file, or the
// standard input when the flag is -.
func readReleaseNotes(notes string) string {
	if notes == "-" {
		data, err := io.ReadAll(feedback.GetStdin())
		if err != nil {
			feedback.Fatal("Cannot read the release notes from the standard input: "+err.Error(), feedback.ErrBadArgument)
		}
		return string(data)
	}

	data, err := paths.New(notes).ReadFile()
	if err != nil {
		feedback.Fatal(fmt.Sprintf("Cannot read the notes file %q: %v", notes, err), feedback.ErrBadArgument)
	}
	return string(data)
}

type buildAppResult struct {
	orchestrator.BuildReleaseResult
	Archive string                        `json:"archive"`
	Output  *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r buildAppResult) String() string {
	return fmt.Sprintf("✓ Release %s built for %s in '%s'", r.Name, r.Target, r.Archive)
}

func (r buildAppResult) Data() any {
	return r
}

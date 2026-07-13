// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package apprelease

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newInstallCmd(cfg config.Configuration) *cobra.Command {
	var force bool
	var noPrepare bool

	cmd := &cobra.Command{
		Use:   "install release_file.tar.gz",
		Short: "Install an Arduino App Release into the apps directory",
		Long: `Install an Arduino App Release (.tar.gz) into the apps directory.

The app is extracted ready to run: the prebuilt sketch, the Python venv, the bundled AI
models and a localized version-pinned compose file are placed on disk, and the app is
flagged as a release in app.yaml. From then on it behaves like a regular app — launch it
with the usual 'arduino-app-cli app start <app>' (it is not recompiled nor re-provisioned).

By default, install also runs the 'prepare' step: it pre-pulls the container images the app
needs (per its compose file) so that the first 'app start' finds them locally. The app is
not started. Use --no-prepare to skip this and run 'arduino-app-cli release prepare <app>'
separately later (a plain 'app start' would also pull any missing images).

Use '-' as release_file to read the release from stdin.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			tarPath, cleanup, err := parseReleasePath(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			defer cleanup()

			installHandler(cmd.Context(), cfg, tarPath, force, !noPrepare)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Install even if the release targets a different board")
	cmd.Flags().BoolVar(&noPrepare, "no-prepare", false, "Skip pre-pulling the app's container images (run 'release prepare' later)")

	return cmd
}

func parseReleasePath(arg string) (*paths.Path, func(), error) {
	if arg == "-" {
		tmpFile, err := paths.MkTempFile(nil, "app_release_*.tar.gz")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create temporary file: %w", err)
		}
		defer tmpFile.Close()

		if _, err = io.Copy(tmpFile, os.Stdin); err != nil { // nolint:forbidigo
			tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return nil, nil, fmt.Errorf("failed to read from stdin: %w", err)
		}

		return paths.New(tmpFile.Name()), func() {
			tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}, nil
	}

	path := paths.New(arg)
	if !path.Exist() {
		return nil, nil, fmt.Errorf("file not found: %s", arg)
	}
	return path, func() {}, nil
}

func installHandler(ctx context.Context, cfg config.Configuration, tarPath *paths.Path, force bool, prepare bool) {
	idProvider := servicelocator.GetAppIDProvider()
	appID, manifest, err := orchestrator.InstallRelease(ctx, cfg, tarPath, idProvider, servicelocator.GetBricksIndex(), servicelocator.GetModelsIndex(), servicelocator.GetPlatform(), force)
	if err != nil {
		switch {
		case errors.Is(err, orchestrator.ErrIncompatibleRelease):
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		case errors.Is(err, orchestrator.ErrAppAlreadyExists):
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
		case errors.Is(err, orchestrator.ErrBadRequest):
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		default:
			feedback.Fatal(fmt.Sprintf("Install failed: %s", err), feedback.ErrGeneric)
		}
	}

	// The install itself is complete and committed on disk. Pre-pulling the container images
	// is a convenience: a failure here must NOT fail the install (the images will be pulled on
	// the first 'app start', or the user can run 'release prepare' later).
	if prepare {
		installedApp, loadErr := app.Load(appID.ToPath())
		if loadErr != nil {
			feedback.Warnf("Release installed, but could not load it to prepare images: %s", loadErr)
		} else {
			out, _, _ := feedback.OutputStreams()
			prepErr := orchestrator.PrepareRelease(ctx, installedApp, servicelocator.GetDockerClient(), func(message orchestrator.StreamMessage) {
				if message.GetType() == orchestrator.InfoType {
					fmt.Fprintln(out, "[INFO]", message.GetData())
				}
			})
			if prepErr != nil {
				feedback.Warnf("Release installed, but preparing container images failed: %s\n"+
					"Run 'arduino-app-cli release prepare %q' later, or it will be pulled on first start.", prepErr, installedApp.Name)
			}
		}
	}

	result := installReleaseResult{AppID: appID.String()}
	if manifest != nil {
		result.AppName = manifest.AppName
	}
	feedback.PrintResult(result)
}

type installReleaseResult struct {
	AppID   string `json:"app_id"`
	AppName string `json:"app_name,omitempty"`
}

func (r installReleaseResult) String() string {
	id := r.AppID
	if decoded, err := base64.RawURLEncoding.DecodeString(r.AppID); err == nil {
		id = string(decoded)
	}
	// The manifest may be unreadable; fall back to the app id so we never print a blank name
	// or suggest `app start ""`.
	name := r.AppName
	startTarget := r.AppName
	if name == "" {
		name = id
	}
	if startTarget == "" {
		startTarget = id
	}
	return fmt.Sprintf("✓ Release installed.\n  App: %s\n  App ID: %s\n  Start it like any app: arduino-app-cli app start %q", name, id, startTarget)
}

func (r installReleaseResult) Data() any {
	if decoded, err := base64.RawURLEncoding.DecodeString(r.AppID); err == nil {
		return installReleaseResult{AppID: string(decoded), AppName: r.AppName}
	}
	return r
}

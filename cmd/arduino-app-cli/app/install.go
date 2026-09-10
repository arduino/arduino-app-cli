// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newInstallCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install release_path",
		Short: "Install an Arduino App release archive",
		Long: `Install an Arduino App release archive.

A release is installed in the releases dir, apart from the apps, and it is read
only: the python environment and the compose files the build froze are its cache,
so that starting it generates nothing and nothing may change it. It must be built
for this board, and it is named after the release, version included.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			installHandler(cfg, paths.New(args[0]))
			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return []string{strings.TrimPrefix(orchestrator.ReleaseArchiveExt, ".")}, cobra.ShellCompDirectiveFilterFileExt
		},
	}

	return cmd
}

func installHandler(cfg config.Configuration, archive *paths.Path) {
	result, err := orchestrator.InstallRelease(archive, servicelocator.GetAppIDProvider(), cfg, servicelocator.GetPlatform())
	switch {
	case errors.Is(err, orchestrator.ErrAppAlreadyExists):
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	case errors.Is(err, orchestrator.ErrBadRequest):
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	case err != nil:
		feedback.Fatal(fmt.Sprintf("Install failed: %s", err), feedback.ErrGeneric)
	}

	feedback.PrintResult(installAppResult{
		Name:    result.Name,
		Version: result.Release.Version,
		Target:  result.Release.Target,
		AppID:   result.AppID.String(),
		Path:    result.Path.String(),
	})
}

type installAppResult struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Target  string `json:"target"`
	AppID   string `json:"app_id"`
	Path    string `json:"path"`
}

func (r installAppResult) String() string {
	return fmt.Sprintf("✓ Release %s %s installed in '%s'\n  App ID: %s", r.Name, r.Version, r.Path, r.appIDOrEncoded())
}

func (r installAppResult) Data() any {
	result := r
	result.AppID = r.appIDOrEncoded()
	return result
}

// The id is printed as it reads, the same the import does.
func (r installAppResult) appIDOrEncoded() string {
	appID, err := base64.RawURLEncoding.DecodeString(r.AppID)
	if err != nil {
		return r.AppID
	}
	return string(appID)
}

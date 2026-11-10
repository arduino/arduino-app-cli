// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"fmt"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/runnerassets"
)

// newDownloadRunnerAssetsCmd is a developer/CI tool: it is not part of what a board
// needs at runtime, and does not substitute `task download-runner-assets`, which
// remains the entry point used by the Taskfile.
func newDownloadRunnerAssetsCmd() *cobra.Command {
	var dest string
	cmd := &cobra.Command{
		Use:    "download-runner-assets",
		Short:  "Downloads the python runner's static assets from its GitHub release",
		Hidden: true,
		Args:   cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			destPath := paths.New(dest)

			feedback.Printf("Downloading runner assets version %s into %s...", config.RunnerVersion, destPath)
			if err := runnerassets.NewDownloader().Download(cmd.Context(), config.RunnerVersion, destPath); err != nil {
				return fmt.Errorf("failed to download runner assets: %w", err)
			}
			feedback.Printf("Runner assets successfully downloaded to %s.", destPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&dest, "dest", "", "Destination directory the assets are copied into (required)")
	_ = cmd.MarkFlagRequired("dest")

	return cmd
}

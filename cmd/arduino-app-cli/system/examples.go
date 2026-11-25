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
	"github.com/arduino/arduino-app-cli/internal/examplesdownload"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// newDownloadExamplesCmd is a developer/CI tool: it is not part of what a board
// needs at runtime, and does not substitute `task build-deb:clone-examples`, which
// remains the entry point used by the Taskfile.
func newDownloadExamplesCmd() *cobra.Command {
	var dest string
	cmd := &cobra.Command{
		Use:    "download-examples",
		Short:  "Downloads the examples from the app-bricks-examples GitHub release",
		Hidden: true,
		Args:   cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			destPath := paths.New(dest)

			feedback.Printf("Downloading examples version %s into %s...", config.ExamplesVersion, destPath.Join("examples"))
			if err := examplesdownload.NewDownloader().Download(cmd.Context(), config.ExamplesVersion, destPath); err != nil {
				return fmt.Errorf("failed to download examples: %w", err)
			}
			feedback.Printf("Examples successfully downloaded to %s.", destPath.Join("examples"))
			return nil
		},
	}
	cmd.Flags().StringVar(&dest, "dest", "", "Directory the examples folder is created in (required)")
	_ = cmd.MarkFlagRequired("dest")

	return cmd
}

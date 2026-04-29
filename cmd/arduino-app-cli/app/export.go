// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newExportCmd(cfg config.Configuration) *cobra.Command {
	var includeData bool
	var override bool

	cmd := &cobra.Command{
		Use:   "export app_path [output_path]",
		Short: "Export an existing Arduino App to a zip file",
		Long: `Export an existing Arduino App to a zip file.
Use '-' as output_path to write the zip to stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := Load(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}
			var outputPath string
			if len(args) > 1 {
				outputPath = args[1]
			}
			return exportHandler(cmd.Context(), servicelocator.GetBricksIndex(), app, outputPath, includeData, override)
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

	cmd.Flags().BoolVar(&includeData, "include-data", false, "Include data directory in the archive")
	cmd.Flags().BoolVar(&override, "overwrite", false, "Overwrite output file if it exists")

	return cmd
}

func exportHandler(ctx context.Context, bricksIndex *bricksindex.BricksIndex, appToExport app.ArduinoApp, outputDest string, includeData bool, override bool) error {

	zipBytes, originalName, err := orchestrator.ExportAppZip(ctx, bricksIndex, appToExport, includeData)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	ext := filepath.Ext(originalName)
	nameNoExt := strings.TrimSuffix(originalName, ext)
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	defaultFileName := fmt.Sprintf("%s_%s%s", nameNoExt, timestamp, ext)

	if outputDest == "-" {
		w, _, err := feedback.DirectStreams()
		if err != nil {
			feedback.Fatal(fmt.Sprintf("Failed to get output stream: %s", err), feedback.ErrGeneric)
		}
		if _, err := w.Write(zipBytes); err != nil {
			feedback.Fatal(fmt.Sprintf("Failed to write zip to stdout: %s", err), feedback.ErrGeneric)
		}
		return nil
	}

	var finalPath *paths.Path
	if outputDest != "" {
		finalPath = paths.New(outputDest)
		if finalPath.IsDir() {
			finalPath = paths.New(filepath.Join(outputDest, defaultFileName))
		}
	} else {
		finalPath = paths.New(defaultFileName)
	}
	if finalPath.Exist() {
		if !override {
			feedback.Fatal(fmt.Sprintf("File '%s' already exists. Use --overwrite to overwrite.", finalPath), feedback.ErrGeneric)
		}
	}

	if err := finalPath.WriteFile(zipBytes); err != nil {
		feedback.Fatal(fmt.Sprintf("Failed to save zip file: %s", err), feedback.ErrGeneric)
	}

	feedback.PrintResult(exportAppResult{
		Result:  "ok",
		Message: "Export successful",
		AppName: finalPath.String(),
	})

	return nil
}

type exportAppResult struct {
	Result  string `json:"result"`
	Message string `json:"message"`
	AppName string `json:"app_name"`
}

func (r exportAppResult) String() string {
	return fmt.Sprintf("✓ %s to '%s'", r.Message, r.AppName)
}

func (r exportAppResult) Data() any {
	return r
}

// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

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

	var finalPath *paths.Path
	if outputDest != "" {
		p := paths.New(outputDest)
		if p.IsDir() {
			finalPath = paths.New(filepath.Join(outputDest, defaultFileName))
		} else {
			finalPath = p
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

func (r exportAppResult) Data() interface{} {
	return r
}

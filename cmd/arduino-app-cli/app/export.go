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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func newExportCmd() *cobra.Command {
	var includeData bool
	var override bool

	cmd := &cobra.Command{
		Use:   "export app_path [output_path]",
		Short: "Export an existing Arduino App to a zip file",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {

			app, err := Load(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}
			outputPath := ""
			if len(args) > 1 {
				outputPath = args[1]
			}
			return exportHandler(cmd.Context(), app, outputPath, includeData, override)
		},
	}

	cmd.Flags().BoolVar(&includeData, "include-data", false, "Include data directory in the archive")
	cmd.Flags().BoolVar(&override, "override", false, "Overwrite output file if it exists")

	return cmd
}

func exportHandler(ctx context.Context, appToExport app.ArduinoApp, outputDest string, includeData bool, override bool) error {

	zipBytes, originalName, err := orchestrator.ExportAppZip(ctx, appToExport, includeData)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	ext := filepath.Ext(originalName)
	nameNoExt := strings.TrimSuffix(originalName, ext)
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	defaultFileName := fmt.Sprintf("%s_%s%s", nameNoExt, timestamp, ext)

	var finalPath string

	if outputDest == "" {
		finalPath = defaultFileName
	} else {
		info, err := os.Stat(outputDest)
		if err == nil && info.IsDir() {
			finalPath = filepath.Join(outputDest, defaultFileName)
		} else {
			finalPath = outputDest
		}
	}

	if fileExists(finalPath) {
		if !override {
			feedback.Fatal(fmt.Sprintf("File '%s' already exists. Use --override to overwrite.", finalPath), feedback.ErrGeneric)
		}
	}

	if err := os.WriteFile(finalPath, zipBytes, 0600); err != nil {
		feedback.Fatal(fmt.Sprintf("Failed to save zip file: %s", err), feedback.ErrGeneric)
	}

	feedback.PrintResult(exportAppResult{
		Result:  "ok",
		Message: "Export successful",
		AppName: finalPath,
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

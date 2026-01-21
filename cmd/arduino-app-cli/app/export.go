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
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func newExportCmd() *cobra.Command {
	var includeData bool

	cmd := &cobra.Command{
		Use:   "export APP_ID",
		Short: "Export an existing Arduino App to a zip file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appID := args[0]
			return exportHandler(cmd.Context(), appID, includeData)
		},
	}

	cmd.Flags().BoolVar(&includeData, "include-data", false, "Include data directory in the archive")

	return cmd
}

func exportHandler(ctx context.Context, appIDStr string, includeData bool) error {
	id, err := servicelocator.GetAppIDProvider().ParseID(appIDStr)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}

	appToExport, err := app.Load(id.ToPath())
	if err != nil {
		slog.Error("Unable to load the app", "error", err.Error(), "path", id.String())
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	zipBytes, originalName, err := orchestrator.ExportAppZip(ctx, appToExport, includeData)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	ext := filepath.Ext(originalName)
	nameNoExt := strings.TrimSuffix(originalName, ext)
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	finalName := fmt.Sprintf("%s_%s%s", nameNoExt, timestamp, ext)

	if fileExists(finalName) {
		feedback.Fatal(fmt.Sprintf("File '%s' already exists and too many renamed versions exist.", finalName), feedback.ErrGeneric)

	}

	if err := os.WriteFile(finalName, zipBytes, 0600); err != nil {
		feedback.Fatal(fmt.Sprintf("Failed to save zip file: %s", err), feedback.ErrGeneric)
	}

	feedback.PrintResult(exportAppResult{
		Result:  "ok",
		Message: "Export successful",
		AppName: finalName,
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

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

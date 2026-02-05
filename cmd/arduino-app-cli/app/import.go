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

func newImportCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import FILE_PATH",
		Short: "Import an Arduino App from a zip file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			zipPath := paths.New(args[0])
			if !zipPath.Exist() {
				feedback.Fatal(fmt.Sprintf("File not found: %s", zipPath), feedback.ErrBadArgument)
				return nil
			}
			return importHandler(cfg, zipPath)
		},
	}

	return cmd
}

func importHandler(cfg config.Configuration, zipPath *paths.Path) error {
	idProvider := servicelocator.GetAppIDProvider()
	appID, err := orchestrator.ImportAppFromZip(cfg, zipPath, idProvider, zipPath.Base())
	if err != nil {
		switch {
		case errors.Is(err, orchestrator.ErrAppAlreadyExists):
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
		case errors.Is(err, orchestrator.ErrBadRequest) || strings.Contains(err.Error(), "not a valid zip file"):
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		default:
			feedback.Fatal(fmt.Sprintf("Import failed: %s", err), feedback.ErrGeneric)
		}
		return nil
	}

	feedback.PrintResult(importAppResult{
		AppID: appID.String(),
	})

	return nil
}

type importAppResult struct {
	AppID string `json:"app_id"`
}

func (r importAppResult) String() string {
	appIDBytes, err := base64.RawURLEncoding.DecodeString(r.AppID)
	if err != nil {
		return fmt.Sprintf("✓ Import successful.\n  App ID: %s", r.AppID)
	}
	return fmt.Sprintf("✓ Import successful.\n  App ID: %s", appIDBytes)
}

func (r importAppResult) Data() interface{} {
	appIDBytes, err := base64.RawURLEncoding.DecodeString(r.AppID)
	if err != nil {
		return r
	}
	return importAppResult{
		AppID: string(appIDBytes),
	}
}

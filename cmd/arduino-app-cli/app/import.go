// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
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
		Use:   "import zip_path",
		Short: "Import an Arduino App from a zip file",
		Long: `Import an Arduino App from a zip file.
Use '-' as zip_path to read the zip from stdin.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			zipPath, cleanup, err := parseFilePath(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			defer cleanup()

			importHandler(cfg, zipPath)

			return nil
		},
	}

	return cmd
}

func parseFilePath(arg string) (*paths.Path, func(), error) {
	if arg == "-" {
		tmpFile, err := paths.MkTempFile(nil, "app_import_*.zip")
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

func importHandler(cfg config.Configuration, zipPath *paths.Path) {
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
	}

	feedback.PrintResult(importAppResult{
		AppID: appID.String(),
	})
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

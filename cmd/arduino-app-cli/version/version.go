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

package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
)

func NewVersionCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Arduino App CLI",
		Run: func(cmd *cobra.Command, args []string) {
			feedback.PrintResult(versionResult{
				AppName: "Arduino App CLI",
				Version: version,
			})
		},
	}
	return cmd
}

type versionResult struct {
	AppName string `json:"appName"`
	Version string `json:"version"`
}

func (r versionResult) String() string {
	return fmt.Sprintf("%s v%s", r.AppName, r.Version)
}

func (r versionResult) Data() interface{} {
	return r
}

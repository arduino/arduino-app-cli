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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
)

// The actual listening address for the daemon
// is defined in the installation package
const (
	DefaultHostname = "localhost"
	DefaultPort     = "8800"
	ProgramName     = "Arduino App CLI"
)

func NewVersionCmd(clientVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Arduino App CLI",
		Run: func(cmd *cobra.Command, args []string) {
			port, _ := cmd.Flags().GetString("port")

			daemonVersion, err := getDaemonVersion(http.Client{}, port)
			if err != nil {
				feedback.Warnf("Warning: cannot get the running daemon version on %s:%s\n", DefaultHostname, port)
			}

			result := versionResult{
				Name:          ProgramName,
				Version:       clientVersion,
				DaemonVersion: daemonVersion,
			}

			feedback.PrintResult(result)
		},
	}
	cmd.Flags().String("port", DefaultPort, "The daemon network port")
	return cmd
}

func getDaemonVersion(httpClient http.Client, port string) (string, error) {

	httpClient.Timeout = time.Second

	url := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(DefaultHostname, port),
		Path:   "/v1/version",
	}

	resp, err := httpClient.Get(url.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code received")
	}

	var daemonResponse struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&daemonResponse); err != nil {
		return "", err
	}

	return daemonResponse.Version, nil
}

type versionResult struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	DaemonVersion string `json:"daemon_version,omitempty"`
}

func (r versionResult) String() string {
	resultMessage := fmt.Sprintf("%s version %s", ProgramName, r.Version)

	if r.DaemonVersion != "" {
		resultMessage = fmt.Sprintf("%s\ndaemon version: %s",
			resultMessage, r.DaemonVersion)
	}
	return resultMessage
}

func (r versionResult) Data() interface{} {
	return r
}

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

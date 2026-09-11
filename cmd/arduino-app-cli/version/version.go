// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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

// NewVersionCmd creates the version command. pythonRunnerVersion is the Python runner
// image resolved from the CLI environment.
func NewVersionCmd(clientVersion string, pythonRunnerVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Arduino App CLI",
		Run: func(cmd *cobra.Command, args []string) {
			port, _ := cmd.Flags().GetString("port")

			daemon, err := getDaemonVersion(http.Client{}, port)
			if err != nil {
				feedback.Warnf("Warning: cannot get the running daemon version on %s:%s\n", DefaultHostname, port)
			}

			result := versionResult{
				Name:                      ProgramName,
				Version:                   clientVersion,
				PythonRunnerVersion:       pythonRunnerVersion,
				DaemonVersion:             daemon.Version,
				DaemonPythonRunnerVersion: daemon.PythonRunnerVersion,
			}

			feedback.PrintResult(result)
		},
	}
	cmd.Flags().String("port", DefaultPort, "The daemon network port")
	return cmd
}

// daemonVersion mirrors the daemon /v1/version response.
type daemonVersion struct {
	Version             string `json:"version"`
	PythonRunnerVersion string `json:"python_runner_version"`
}

func getDaemonVersion(httpClient http.Client, port string) (daemonVersion, error) {

	httpClient.Timeout = time.Second

	url := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(DefaultHostname, port),
		Path:   "/v1/version",
	}

	resp, err := httpClient.Get(url.String())
	if err != nil {
		return daemonVersion{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return daemonVersion{}, fmt.Errorf("unexpected status code received")
	}

	var daemonResponse daemonVersion
	if err := json.NewDecoder(resp.Body).Decode(&daemonResponse); err != nil {
		return daemonVersion{}, err
	}

	return daemonResponse, nil
}

type versionResult struct {
	Name                      string `json:"name"`
	Version                   string `json:"version"`
	PythonRunnerVersion       string `json:"python_runner_version"`
	DaemonVersion             string `json:"daemon_version,omitempty"`
	DaemonPythonRunnerVersion string `json:"daemon_python_runner_version,omitempty"`
}

func (r versionResult) String() string {
	resultMessage := fmt.Sprintf("%s version %s\npython runner version: %s", ProgramName, r.Version, r.PythonRunnerVersion)

	if r.DaemonVersion != "" {
		resultMessage = fmt.Sprintf("%s\ndaemon version: %s",
			resultMessage, r.DaemonVersion)
	}
	if r.DaemonPythonRunnerVersion != "" {
		resultMessage = fmt.Sprintf("%s\ndaemon python runner version: %s",
			resultMessage, r.DaemonPythonRunnerVersion)
	}
	return resultMessage
}

func (r versionResult) Data() any {
	return r
}

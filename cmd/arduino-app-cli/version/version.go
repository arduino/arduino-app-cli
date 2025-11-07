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
	"strings"
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
		Short: "Print the client and server version numbers for the Arduino App CLI.",
		Run: func(cmd *cobra.Command, args []string) {
			host, _ := cmd.Flags().GetString("host")

			validatedHostAndPort, err := validateHost(host)
			if err != nil {
				feedback.Fatal("Error: invalid host:port format", feedback.ErrBadArgument)
			}

			httpClient := http.Client{
				Timeout: time.Second,
			}

			result, err := versionHandler(httpClient, clientVersion, validatedHostAndPort)
			if err != nil {
				feedback.Warnf("Waning: " + err.Error() + "\n")
			}
			feedback.PrintResult(result)
		},
	}
	cmd.Flags().String("host", fmt.Sprintf("%s:%s", DefaultHostname, DefaultPort),
		"The daemon network address [host]:[port]")
	return cmd
}

func versionHandler(httpClient http.Client, clientVersion string, hostAndPort string) (versionResult, error) {
	url := url.URL{
		Scheme: "http",
		Host:   hostAndPort,
		Path:   "/v1/version",
	}

	daemonVersion := getServerVersion(httpClient, url.String())

	result := versionResult{
		Name:          ProgramName,
		ClientVersion: clientVersion,
		DaemonVersion: daemonVersion,
	}

	if daemonVersion == "" {
		return result, fmt.Errorf("cannot connect to %s", hostAndPort)
	}
	return result, nil
}

func validateHost(hostPort string) (string, error) {
	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":"
	}

	h, p, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", err
	}
	if h == "" {
		h = DefaultHostname
	}
	if p == "" {
		p = DefaultPort
	}

	return net.JoinHostPort(h, p), nil
}

func getServerVersion(httpClient http.Client, url string) string {
	resp, err := httpClient.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var serverResponse struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&serverResponse); err != nil {
		return ""
	}

	return serverResponse.Version
}

type versionResult struct {
	Name          string `json:"name"`
	ClientVersion string `json:"client_version"`
	DaemonVersion string `json:"daemon_version,omitempty"`
}

func (r versionResult) String() string {
	serverMessage := fmt.Sprintf("%s client version %s",
		ProgramName, r.ClientVersion)

	if r.DaemonVersion != "" {
		serverMessage = fmt.Sprintf("%s\ndaemon version: %s",
			serverMessage, r.DaemonVersion)
	}
	return serverMessage
}

func (r versionResult) Data() interface{} {
	return r
}

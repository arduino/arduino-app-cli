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
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/cmd/i18n"
)

// The actual listening address for the daemon
// is defined in the installation package
const (
	DefaultHostname = "localhost"
	DefaultPort     = "8800"
)

func NewVersionCmd(clientVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the client and server version numbers for the Arduino App CLI.",
		Run: func(cmd *cobra.Command, args []string) {
			host, _ := cmd.Flags().GetString("host")

			versionHandler(clientVersion, host)
		},
	}
	cmd.Flags().String("host", fmt.Sprintf("%s:%s", DefaultHostname, DefaultPort),
		"The daemon network address [host]:[port]")
	return cmd
}

func versionHandler(clientVersion string, host string) {
	httpClient := http.Client{
		Timeout: time.Second,
	}
	result := doVersionHandler(httpClient, clientVersion, host)
	feedback.PrintResult(result)
}

func doVersionHandler(httpClient http.Client, clientVersion string, host string) versionResult {
	url, err := getValidOrDefaultUrl(host)
	if err != nil {
		feedback.Fatal(i18n.Tr("Error: invalid host:port format"), feedback.ErrBadArgument)
	}

	serverVersion, err := getServerVersion(httpClient, url)
	if err != nil {
		serverVersion = fmt.Sprintf("n/a (cannot connect to the server %s://%s)", url.Scheme, url.Host)
	}

	return versionResult{
		ClientVersion: clientVersion,
		ServerVersion: serverVersion,
	}
}

func getValidOrDefaultUrl(hostPort string) (url.URL, error) {
	host := DefaultHostname
	port := DefaultPort

	if hostPort != "" {
		h, p, err := net.SplitHostPort(hostPort)
		if err != nil {
			return url.URL{}, err
		}
		if h != "" {
			host = h
		}
		if p != "" {
			port = p
		}

	}

	hostAndPort := net.JoinHostPort(host, port)

	u := url.URL{
		Scheme: "http",
		Host:   hostAndPort,
		Path:   "/v1/version",
	}

	return u, nil
}

func getServerVersion(httpClient http.Client, url url.URL) (string, error) {
	resp, err := httpClient.Get(url.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var serverResponse serverVersionResponse
	if err := json.Unmarshal(body, &serverResponse); err != nil {
		return "", err
	}

	return serverResponse.Version, nil
}

type serverVersionResponse struct {
	Version string `json:"version"`
}

type versionResult struct {
	ClientVersion string `json:"version"`
	ServerVersion string `json:"serverVersion"`
}

func (r versionResult) String() string {
	return fmt.Sprintf("client: %s\nserver: %s",
		r.ClientVersion, r.ServerVersion)
}

func (r versionResult) Data() interface{} {
	return r
}

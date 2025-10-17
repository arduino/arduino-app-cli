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

package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/fatih/color"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

//go:generate go tool oapi-codegen -config cfg.yaml ../api/docs/openapi.yaml

type ArduinoAppCLI struct {
	t            *require.Assertions
	DaemonAddr   string
	path         *paths.Path
	appDir       *paths.Path
	configDir    *paths.Path
	envVars      map[string]string
	proc         *paths.Process
	stdIn        io.WriteCloser
	daemonClient *client.Client
}

func NewArduinoAppCLI(t *testing.T) *ArduinoAppCLI {
	rootDir, err := paths.MkTempDir("", "app-cli")
	require.NoError(t, err)
	appDir := rootDir.Join("ArduinoApps")
	dataDir := rootDir.Join("data")
	configDir := rootDir.Join("config")
	originalTestDataDir := FindRepositoryRootPath(t).Join("internal", "e2e", "daemon", "testdata")
	if originalTestDataDir.Exist() {
		require.NoError(t, os.CopyFS(dataDir.String(), os.DirFS(originalTestDataDir.String())))
		require.NoError(t, err, "failed to copy testdata to temp dir")
	}

	return &ArduinoAppCLI{
		t:          require.New(t),
		DaemonAddr: "",
		path:       FindArduinoAppCLIPath(t),
		appDir:     appDir,
		configDir:  configDir,
		envVars: map[string]string{
			"ARDUINO_APP_CLI__APPS_DIR":   appDir.String(),
			"ARDUINO_APP_CLI__CONFIG_DIR": configDir.String(),
			"ARDUINO_APP_CLI__DATA_DIR":   dataDir.String(),
		},
	}
}

// FindRepositoryRootPath returns the repository root path
func FindRepositoryRootPath(t *testing.T) *paths.Path {
	repoRootPath, err := paths.Getwd()
	require.NoError(t, err)
	for !repoRootPath.Join(".git").Exist() {
		t.Log(repoRootPath.String())
		require.Contains(t, repoRootPath.String(), "arduino-app-cli", "Error searching for repository root path")
		repoRootPath = repoRootPath.Parent()
	}
	return repoRootPath
}

// FindArduinoAppCLIPath returns the path to the arduino-cli executable
func FindArduinoAppCLIPath(t *testing.T) *paths.Path {
	return FindRepositoryRootPath(t).Join("arduino-app-cli")
}

// CreateEnvForDaemon performs the minimum required operations to start the arduino-app-cli daemon.
// It returns a testsuite.Environment and an ArduinoAppCLI client to perform the integration tests.
// The Environment must be disposed by calling the CleanUp method via defer.
func CreateEnvForDaemon(t *testing.T) *ArduinoAppCLI {
	cli := NewArduinoAppCLI(t)
	_ = cli.StartDaemon(false)
	return cli
}

func (cli *ArduinoAppCLI) StartDaemon(verbose bool) string {
	args := []string{"daemon"}
	cliProc, err := paths.NewProcessFromPath(cli.convertEnvForExecutils(cli.envVars), cli.path, args...)
	cli.t.NoError(err)
	stdout, err := cliProc.StdoutPipe()
	cli.t.NoError(err)
	stderr, err := cliProc.StderrPipe()
	cli.t.NoError(err)
	stdIn, err := cliProc.StdinPipe()
	cli.t.NoError(err)

	cli.t.NoError(cliProc.Start())
	cli.stdIn = stdIn
	cli.proc = cliProc
	cli.DaemonAddr = "http://127.0.0.1:8080"

	_copy := func(dst io.Writer, src io.Reader) {
		buff := make([]byte, 1024)
		for {
			n, err := src.Read(buff)
			if err != nil {
				return
			}
			_, _ = dst.Write([]byte(color.YellowString(string(buff[:n]))))
		}
	}
	go _copy(os.Stdout, stdout)
	go _copy(os.Stderr, stderr)

	// Await the CLI daemon to be ready
	var connErr error
	for range 10 {
		time.Sleep(time.Second)

		c, err := client.NewClient(cli.DaemonAddr)
		if err != nil {
			connErr = err
			continue
		}
		r, err := c.GetApps(context.Background(), nil)
		if err != nil {
			connErr = err
			continue
		}
		_ = r.Body.Close()
		if r.StatusCode != http.StatusOK {
			continue
		}

		cli.daemonClient = c
		break
	}
	cli.t.NoError(connErr)
	return cli.DaemonAddr
}

// convertEnvForExecutils returns a string array made of "key=value" strings
// with (key,value) pairs obtained from the given map.
func (cli *ArduinoAppCLI) convertEnvForExecutils(env map[string]string) []string {
	envVars := []string{}
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Proxy code-coverage related env vars
	if gocoverdir := os.Getenv("INTEGRATION_GOCOVERDIR"); gocoverdir != "" {
		envVars = append(envVars, "GOCOVERDIR="+gocoverdir)
	}
	return envVars
}

// CleanUp closes the Arduino App CLI client.
func (cli *ArduinoAppCLI) CleanUp() {
	if cli.proc != nil {
		cli.stdIn.Close()
		proc := cli.proc
		go func() {
			time.Sleep(time.Second)
			_ = proc.Kill()
		}()
		_ = cli.proc.Wait()
	}

	cli.t.NoError(cli.appDir.Parent().RemoveAll())
}

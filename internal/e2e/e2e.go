// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package e2e

import (
	"bufio"
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
	testT        *testing.T
	DaemonAddr   string
	path         *paths.Path
	appDir       *paths.Path
	envVars      map[string]string
	proc         *paths.Process
	stdIn        io.WriteCloser
	daemonClient *client.Client
}

// ArduinoAppCLIOption allows customizing ArduinoAppCLI construction
type ArduinoAppCLIOption func(*ArduinoAppCLI)

// WithCustomModelDir sets a custom model directory in envVars
func WithCustomModelDir(dir *paths.Path) ArduinoAppCLIOption {
	return func(cli *ArduinoAppCLI) {
		if dir != nil {
			cli.envVars["ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR"] = dir.String()
		}
	}
}

// WithModelDir sets a custom model directory in envVars
func WithModelsDir(dir *paths.Path) ArduinoAppCLIOption {
	return func(cli *ArduinoAppCLI) {
		if dir != nil {
			cli.envVars["MODELS_PATH"] = dir.String()
		}
	}
}

func WithBoardName(name string) ArduinoAppCLIOption {
	return func(cli *ArduinoAppCLI) {
		content := fmt.Sprintf(`{"board_name":%q}`, name)
		dataDir := paths.New(cli.envVars["ARDUINO_APP_CLI__DATA_DIR"])
		cli.t.NoError(dataDir.Join("platform.json").WriteFile([]byte(content)))
	}
}

func NewArduinoAppCLI(t *testing.T, opts ...ArduinoAppCLIOption) *ArduinoAppCLI {
	rootDir, err := paths.MkTempDir("", "app-cli")
	require.NoError(t, err)
	appDir := rootDir.Join("ArduinoApps")
	dataDir := rootDir.Join("data")
	originalTestDataDir := FindRepositoryRootPath(t).Join("internal", "e2e", "daemon", "testdata")
	if originalTestDataDir.Exist() {
		require.NoError(t, os.CopyFS(dataDir.String(), os.DirFS(originalTestDataDir.String())))
		require.NoError(t, err, "failed to copy testdata to temp dir")
	}
	require.NoError(t, dataDir.Join("models").MkdirAll())

	cli := &ArduinoAppCLI{
		t:          require.New(t),
		testT:      t,
		DaemonAddr: "",
		path:       FindArduinoAppCLIPath(t),
		appDir:     appDir,
		envVars: map[string]string{
			"ARDUINO_APP_CLI__APPS_DIR": appDir.String(),
			"ARDUINO_APP_CLI__DATA_DIR": dataDir.String(),
			// allow ci to run cli with whatever user it wants.
			"ARDUINO_APP_CLI__ALLOW_ROOT": "true",
		},
	}
	for _, opt := range opts {
		opt(cli)
	}
	return cli
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
func CreateEnvForDaemon(t *testing.T, opts ...ArduinoAppCLIOption) *ArduinoAppCLI {
	cli := NewArduinoAppCLI(t, opts...)
	_ = cli.StartDaemon()
	return cli
}

func (cli *ArduinoAppCLI) StartDaemon() string {
	args := []string{"daemon", "--port", "6789", "--log-level", "debug"}
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
	cli.DaemonAddr = "http://127.0.0.1:6789"

	// Forward daemon stdout/stderr to t.Log so it only surfaces on test failure (or with -v).
	logDaemon := func(prefix string, src io.Reader) {
		scanner := bufio.NewScanner(src)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			cli.testT.Log(color.YellowString(prefix + scanner.Text()))
		}
	}
	go logDaemon("daemon stdout: ", stdout)
	go logDaemon("daemon stderr: ", stderr)

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

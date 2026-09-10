// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/moby/moby/client/pkg/versions"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
)

func newCheckCmd(docker command.Cli) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify the board provides what the apps need of it",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			result := checkResult{Checks: checkPlatform(cmd.Context(), docker)}
			feedback.PrintResult(result)
			if failed := result.failed(); failed > 0 {
				return fmt.Errorf("this board does not provide %d of the %d requirements", failed, len(result.Checks))
			}
			return nil
		},
	}
}

// checkPlatform reports what this board provides of what an app needs of it. A new
// requirement is a new entry here, and the command reports it without knowing it.
func checkPlatform(ctx context.Context, docker command.Cli) []check {
	checks := []struct {
		name string
		run  func() (string, error)
	}{
		{"docker engine", func() (string, error) { return checkDockerEngine(ctx, docker) }},
	}

	results := make([]check, 0, len(checks))
	for _, c := range checks {
		detail, err := c.run()
		result := check{Name: c.name, Detail: detail}
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}

// minEngineAPI is the docker engine this cli is built for, stated in the README: the
// compose library it links branches at 1.44, and below it takes paths we do not test.
const minEngineAPI = "1.44"

// checkDockerEngine is the only thing that states the engine we need: the client
// negotiates whatever version the board offers, older ones included.
func checkDockerEngine(ctx context.Context, docker command.Cli) (string, error) {
	version, apiVersion, err := dockerhelper.EngineVersion(ctx, docker)
	if err != nil {
		return "", err
	}
	detail := fmt.Sprintf("version %s, api %s", version, apiVersion)
	if versions.LessThan(apiVersion, minEngineAPI) {
		return detail, fmt.Errorf("the engine speaks api %s, arduino-app-cli needs api %s or later", apiVersion, minEngineAPI)
	}
	return detail, nil
}

// check is what one requirement of a board turned out to be.
type check struct {
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

type checkResult struct {
	Checks []check `json:"checks"`
}

func (r checkResult) String() string {
	var report strings.Builder
	for _, c := range r.Checks {
		state := "ok"
		if c.Error != "" {
			state = c.Error
		}
		fmt.Fprintf(&report, "%-16s %s", c.Name, state)
		if c.Detail != "" {
			report.WriteString(" (" + c.Detail + ")")
		}
		report.WriteString("\n")
	}
	return strings.TrimRight(report.String(), "\n")
}

func (r checkResult) Data() any { return r }

func (r checkResult) failed() int {
	var failed int
	for _, c := range r.Checks {
		if c.Error != "" {
			failed++
		}
	}
	return failed
}

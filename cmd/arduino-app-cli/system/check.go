// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"context"
	"fmt"
	"os/user"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/moby/moby/client/pkg/versions"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/sudo"

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
		{"sudo rules", func() (string, error) { return checkSudoRules(ctx) }},
		{"user groups", checkUserGroups},
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

// checkSudoRules reports the commands of the cli the sudoers file does not let the
// daemon user run without a password.
func checkSudoRules(ctx context.Context) (string, error) {
	missing, err := sudo.Check(ctx)
	if err != nil {
		return "", err
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("the sudoers file does not allow without a password: %s", strings.Join(missing, "; "))
	}
	return "every command of the cli is allowed without a password", nil
}

// requiredGroups are what an app may need of the host: the hardware and privileges its
// containers reach through a bind mount or a socket the current user must belong to.
var requiredGroups = []string{
	"arduino", "adm", "dialout", "audio", "video", "users",
	"bluetooth", "netdev", "fastrpc", "docker", "sysupgrade", "gpiod",
	"render", "input",
}

// checkUserGroups reports the groups of requiredGroups the current user does not
// belong to, whether because the group does not exist on this board or because the
// user was never added to it.
func checkUserGroups() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("cannot determine the current user: %w", err)
	}

	groupIDs, err := current.GroupIds()
	if err != nil {
		return "", fmt.Errorf("cannot list the groups of %s: %w", current.Username, err)
	}
	memberOf := make(map[string]bool, len(groupIDs))
	for _, gid := range groupIDs {
		memberOf[gid] = true
	}

	var missing []string
	for _, name := range requiredGroups {
		if group, err := user.LookupGroup(name); err != nil || !memberOf[group.Gid] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("run ` usermod -aG %s %s` and start a new session", strings.Join(missing, ","), current.Username)
	}
	return fmt.Sprintf("%s belongs to every required group", current.Username), nil
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

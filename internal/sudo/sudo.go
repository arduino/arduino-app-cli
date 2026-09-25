// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package sudo holds every command the cli runs with sudo, and checks the rules
// of the sudoers file it ships, debian/arduino-app-cli/etc/sudoers.d.
package sudo

import (
	"context"
	"fmt"
	"os/user"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
)

// Command is one command the cli runs with sudo. Samples are tails a rule ending
// with * allows, and Env needs an env_keep.
type Command struct {
	Args    []string
	Samples [][]string
	Env     []string
}

// Commands is filled by add, so it cannot fall behind the declarations.
var Commands []Command

// debianFrontend keeps debconf away from the terminal. sudo gives every command a
// pty, so dpkg-preconfigure would open /dev/tty and wait there for ever.
const debianFrontend = "DEBIAN_FRONTEND=noninteractive"

// The commands of `system update`.
var (
	AptUpdate = add(Command{
		Args: []string{"apt-get", "update"},
		Env:  []string{debianFrontend},
	})
	AptUpgrade = add(Command{
		Args:    []string{"apt-get", "install", "--only-upgrade", "-y"},
		Samples: [][]string{{"--allow-downgrades", "arduino-app-cli"}},
		Env:     []string{debianFrontend, "NEEDRESTART_MODE=a"},
	})
	AptClean = add(Command{
		Args: []string{"apt-get", "clean", "-y"},
		Env:  []string{debianFrontend},
	})
	AptLockProbe = add(Command{
		Args: []string{"apt-get", "install", "--assume-no", "non-existent-package-probe"},
		Env:  []string{debianFrontend},
	})
	DpkgConfigure = add(Command{
		Args: []string{"dpkg", "--configure", "-a"},
		Env:  []string{debianFrontend},
	})
	Needrestart = add(Command{
		Args: []string{"needrestart", "-r", "a"},
		Env:  []string{"NEEDRESTART_MODE=a"},
	})
)

// The command of `system init`: a rule names the package, so a new board needs
// a new sample.
var AptInstall = add(Command{
	Args:    []string{"apt-get", "install", "-y"},
	Samples: [][]string{{"arduino-unoq"}, {"arduino-ventunoq"}},
	Env:     []string{debianFrontend},
})

// The command of `system set-name`, which pkg/board still spells out itself.
var SetHostname = add(Command{
	Args:    []string{"hostnamectl", "set-hostname"},
	Samples: [][]string{{"unoq"}},
})

// Process builds the process of the command, with tail appended to its arguments.
func (c Command) Process(tail ...string) (*paths.Process, error) {
	return paths.NewProcess(c.Env, slices.Concat([]string{"sudo"}, c.Args, tail)...)
}

// Check reports what the sudoers file does not allow to the daemon user. `sudo -l`
// resolves a rule and runs nothing.
func Check(ctx context.Context) ([]string, error) {
	// -U reads the rules of the daemon user: sudo allows root everything.
	const daemonUID = "1000"
	daemon, err := user.LookupId(daemonUID)
	if err != nil {
		return nil, fmt.Errorf("no user with uid %s on this system: %w", daemonUID, err)
	}

	sudoList := func(args ...string) ([]byte, error) {
		cmd, err := paths.NewProcess(nil,
			slices.Concat([]string{"sudo", "-n", "-l", "-U", daemon.Username}, args)...)
		if err != nil {
			return nil, err
		}
		return cmd.RunAndCaptureCombinedOutput(ctx)
	}

	// The Defaults of the user name the variables sudo keeps.
	kept, err := sudoList()
	if err != nil {
		return nil, fmt.Errorf("cannot list the sudo rules of %s: %w: %s", daemon.Username, err, kept)
	}

	var missing, checked []string
	for _, c := range Commands {
		samples := c.Samples
		if samples == nil {
			samples = [][]string{nil}
		}
		for _, sample := range samples {
			argv := slices.Concat(c.Args, sample)
			if _, err := sudoList(argv...); err != nil {
				missing = append(missing, strings.Join(argv, " "))
			}
		}
		for _, env := range c.Env {
			name, _, _ := strings.Cut(env, "=")
			if slices.Contains(checked, name) {
				continue
			}
			checked = append(checked, name)
			if !strings.Contains(string(kept), name) {
				missing = append(missing, "the variable "+name)
			}
		}
	}
	return missing, nil
}

func add(c Command) Command {
	Commands = append(Commands, c)
	return c
}

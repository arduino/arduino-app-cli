// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/moby/moby/api/types/container"
)

type Status string

const (
	StatusStarting      Status = "starting"
	StatusRunning       Status = "running"
	StatusStopping      Status = "stopping"
	StatusStopped       Status = "stopped"
	StatusFailed        Status = "failed"
	StatusUninitialized Status = "uninitialized"
)

func StatusFromDockerState(s container.ContainerState, statusMessage string, serviceName string) Status {
	switch s {
	case container.StateRunning:
		return StatusRunning
	case container.StateRestarting:
		return StatusStarting
	case container.StateRemoving:
		return StatusStopping
	case container.StateCreated, container.StatePaused:
		return StatusStopped
	case container.StateExited:
		// Containers without the compose service label keep the previous (main) behavior.
		if serviceName == "" {
			serviceName = MainServiceName
		}
		exitCode, ok := parseExitCode(statusMessage)
		if !ok {
			return StatusFailed
		}
		if serviceName == MainServiceName {
			// POSIX exit code greater than 128+n means terminated by signal https://tldp.org/LDP/abs/html/exitcodes.html
			if exitCode > 128 {
				return StatusStopped
			}
			// The main service exited on its own, which we consider a failure.
			return StatusFailed
		}
		// Non-main services: exit code 0 (completed successfully) or > 128 (terminated by signal) is considered stopped.
		if exitCode == 0 || exitCode > 128 {
			return StatusStopped
		}
		return StatusFailed
	case container.StateDead:
		return StatusFailed
	default:
		panic("unreachable")
	}
}

func ParseStatus(s string) (Status, error) {
	s1 := Status(s)
	return s1, s1.Validate()
}

func (s Status) Validate() error {
	switch s {
	case StatusStarting, StatusRunning, StatusStopping, StatusStopped, StatusFailed, StatusUninitialized:
		return nil
	default:
		return fmt.Errorf("status should be one of %v", s.AllowedStatuses())
	}
}

func (s Status) AllowedStatuses() []Status {
	return []Status{StatusStarting, StatusRunning, StatusStopping, StatusStopped, StatusFailed, StatusUninitialized}
}

var exitCodeRegex = regexp.MustCompile(`Exited \((\d+)\)`)

func parseExitCode(statusMessage string) (int, bool) {
	matches := exitCodeRegex.FindStringSubmatch(statusMessage)
	if len(matches) < 2 {
		// not matching an exit code
		return 0, false
	}
	exitCode, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return exitCode, true
}

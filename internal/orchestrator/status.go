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

func StatusFromDockerState(s container.ContainerState, statusMessage string, isSupportingService bool) Status {
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
		code, ok := exitCodeFromDockerStatus(statusMessage)
		if ok && (code > 128 || (isSupportingService && code == 0)) {
			return StatusStopped
		}
		// A main process exiting on its own is considered an app failure.
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

var dockerExitCodeRegex = regexp.MustCompile(`^Exited \((\d+)\)`)

func exitCodeFromDockerStatus(message string) (int, bool) {
	matches := dockerExitCodeRegex.FindStringSubmatch(message)
	if len(matches) < 2 {
		return 0, false
	}
	code, err := strconv.Atoi(matches[1])
	return code, err == nil
}

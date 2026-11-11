// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"regexp"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/moby/moby/api/types/container"
	"go.bug.st/f"
)

type Status = appbook.Status

const (
	StatusStarting      = appbook.StatusStarting
	StatusRunning       = appbook.StatusRunning
	StatusStopping      = appbook.StatusStopping
	StatusStopped       = appbook.StatusStopped
	StatusFailed        = appbook.StatusFailed
	StatusUninitialized = appbook.StatusUninitialized
)

var ParseStatus = appbook.ParseStatus

func StatusFromDockerState(s container.ContainerState, statusMessage string) Status {
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
		if !isExitBySignal(statusMessage) {
			// The app exited on its own, which we consider a failure.
			return StatusFailed
		}
		return StatusStopped
	case container.StateDead:
		return StatusFailed
	default:
		panic("unreachable")
	}
}

func isExitBySignal(statusMessage string) bool {
	var exitCodeRegex = regexp.MustCompile(`Exited \((\d+)\)`)
	matches := exitCodeRegex.FindStringSubmatch(statusMessage)
	if len(matches) < 2 {
		// not matching an exit code
		return false
	}
	exitCode := f.Must(strconv.Atoi(matches[1]))

	// posix exit code greater than 128+n means terminated by signal https://tldp.org/LDP/abs/html/exitcodes.html
	return exitCode > 128

}

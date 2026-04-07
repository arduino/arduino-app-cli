// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package orchestrator

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"go.bug.st/f"
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

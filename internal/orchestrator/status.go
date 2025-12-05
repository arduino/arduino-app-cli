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

package orchestrator

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
)

type State string

const (
	StatusStarting State = "starting"
	StatusRunning  State = "running"
	StatusStopping State = "stopping"
	StatusStopped  State = "stopped"
	StatusFailed   State = "failed"
)

func StatusFromDockerState(s container.ContainerState) State {
	switch s {
	case container.StateRunning:
		return StatusRunning
	case container.StateRestarting:
		return StatusStarting
	case container.StateRemoving:
		return StatusStopping
	case container.StateCreated, container.StateExited, container.StatePaused:
		return StatusStopped
	case container.StateDead:
		return StatusFailed
	default:
		panic("unreachable")
	}
}

func ParseStatus(s string) (State, error) {
	s1 := State(s)
	return s1, s1.Validate()
}

func (s State) Validate() error {
	switch s {
	case StatusStarting, StatusRunning, StatusStopping, StatusStopped, StatusFailed:
		return nil
	default:
		return fmt.Errorf("status should be one of %v", s.AllowedStatuses())
	}
}

func (s State) AllowedStatuses() []State {
	return []State{StatusStarting, StatusRunning, StatusStopping, StatusStopped, StatusFailed}
}

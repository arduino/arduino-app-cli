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

package update

import "go.bug.st/f"

// EventType defines the type of upgrade event.
type EventType int

const (
	UpgradeLineEvent EventType = iota
	StartEvent
	RestartEvent
	DoneEvent
	ErrorEvent
)

// Event represents a single event in the upgrade process.
type Event struct {
	Type EventType

	data string
	err  error // error field for error events
}

func (t EventType) String() string {
	switch t {
	case UpgradeLineEvent:
		return "log"
	case RestartEvent:
		return "restarting"
	case StartEvent:
		return "starting"
	case DoneEvent:
		return "done"
	case ErrorEvent:
		return "error"
	default:
		panic("unreachable")
	}
}

func NewDataEvent(t EventType, data string) Event {
	return Event{
		Type: t,
		data: data,
	}
}

func NewErrorEvent(err error) Event {
	return Event{
		Type: ErrorEvent,
		err:  err,
	}
}

func (e Event) GetData() string {
	f.Assert(e.Type != ErrorEvent, "not a data event")
	return e.data
}

func (e Event) GetError() error {
	f.Assert(e.Type == ErrorEvent, "not an error event")
	return e.err
}

type PackageType string

const (
	Arduino PackageType = "arduino-platform"
	Debian  PackageType = "debian-package"
)

func (s PackageType) AllowedStatuses() []PackageType {
	return []PackageType{Arduino, Debian}
}

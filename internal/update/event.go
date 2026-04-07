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

type EventCallback func(Event)

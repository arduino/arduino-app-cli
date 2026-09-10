// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import "go.bug.st/f"

// EventType defines the type of upgrade event.
type EventType int

const (
	UpgradeLineEvent EventType = iota
	StartEvent
	RestartEvent
	ProgressEvent
	DoneEvent
	ErrorEvent
)

// Event represents a single event in the upgrade process.
type Event struct {
	Type     EventType
	progress *ProgressInfo
	data     string
	err      error // error field for error events
}

// ProgressInfo carries the completion percentage of the update process, together
// with the step currently being executed.
type ProgressInfo struct {
	Step     string  `json:"step"`
	Progress float32 `json:"progress"`
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
	case ProgressEvent:
		return "progress"
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

func NewProgressEvent(step string, progress float32) Event {
	return Event{
		Type: ProgressEvent,
		progress: &ProgressInfo{
			Step:     step,
			Progress: progress,
		},
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

func (e Event) GetProgress() ProgressInfo {
	f.Assert(e.Type == ProgressEvent, "not a progress event")
	return *e.progress
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

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"fmt"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

const (
	initLogEventType      = "log"
	initProgressEventType = "progress"
)

var _ feedback.Result = (*initResult)(nil)

type initResult struct {
	Type    string `json:"type"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message,omitempty"`
	Label   string `json:"label,omitempty"`
	Current int64  `json:"current,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Percent int    `json:"percent,omitempty"`
}

// Data implements feedback.Result.
func (e *initResult) Data() any {
	return e
}

// String implements feedback.Result.
func (e *initResult) String() string {
	if e.Type == initProgressEventType {
		return fmt.Sprintf("%s: %d%%", e.Label, e.Percent)
	}
	return e.Message
}

func fromInitEvent(e orchestrator.InitEvent) *initResult {
	switch e.Type {
	case orchestrator.InitLogEvent:
		return &initResult{
			Type:    initLogEventType,
			Source:  string(e.Source),
			Message: e.Message,
		}
	case orchestrator.InitProgressEvent:
		p := e.Progress
		return &initResult{
			Type:    initProgressEventType,
			Source:  string(e.Source),
			Label:   p.Label,
			Current: p.Curr,
			Total:   p.Total,
			Percent: percent(p.Curr, p.Total),
		}
	default:
		return nil
	}
}

func percent(curr, total int64) int {
	if total <= 0 {
		return 0
	}
	return int(float64(curr) / float64(total) * 100)
}

func newInitEventCallback(printEvent func(feedback.Result)) orchestrator.InitEventCallback {
	return throttleProgress(func(e *initResult) {
		printEvent(e)
	})
}

// throttleProgress drops the progress events that would render the same
// percentage twice in a row for a given label.
func throttleProgress(next func(*initResult)) orchestrator.InitEventCallback {
	lastPct := map[string]int{}
	return func(event orchestrator.InitEvent) {
		e := fromInitEvent(event)
		if e == nil {
			return
		}
		if e.Type == initProgressEventType {
			if e.Total <= 0 {
				return
			}
			if last, ok := lastPct[e.Label]; ok && last == e.Percent {
				return
			}
			lastPct[e.Label] = e.Percent
		}
		next(e)
	}
}

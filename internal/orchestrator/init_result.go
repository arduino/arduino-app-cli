// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import "fmt"

// Types of an InitResult.
const (
	InitResultLog      = "log"
	InitResultProgress = "progress"
)

// InitResult is the serializable form of an InitEvent: it is what the
// `system init` command writes, one JSON object per line, when asked for the
// json-lines output format.
//
// It is a contract, not just a rendering: the update process runs `system init`
// as a subprocess and reads these lines back to follow the progress of the
// docker images download, so the field names are part of an interface between
// the two.
type InitResult struct {
	Type    string `json:"type"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message,omitempty"`
	Label   string `json:"label,omitempty"`
	Current int64  `json:"current,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Percent int    `json:"percent,omitempty"`
}

// Data implements feedback.Result.
func (e *InitResult) Data() any {
	return e
}

// String implements feedback.Result.
func (e *InitResult) String() string {
	if e.Type == InitResultProgress {
		return fmt.Sprintf("%s: %d%%", e.Label, e.Percent)
	}
	return e.Message
}

// NewInitResult converts an event into its serializable form, and reports nil
// for the event types that have none.
func NewInitResult(e InitEvent) *InitResult {
	switch e.Type {
	case InitLogEvent:
		return &InitResult{
			Type:    InitResultLog,
			Source:  string(e.Source),
			Message: e.Message,
		}
	case InitProgressEvent:
		p := e.Progress
		return &InitResult{
			Type:    InitResultProgress,
			Source:  string(e.Source),
			Label:   p.Label,
			Current: p.Curr,
			Total:   p.Total,
			Percent: initPercent(p.Curr, p.Total),
		}
	default:
		return nil
	}
}

func initPercent(curr, total int64) int {
	if total <= 0 {
		return 0
	}
	return int(float64(curr) / float64(total) * 100)
}

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import "fmt"

type InitResultType string

const (
	InitResultLog      InitResultType = "log"
	InitResultProgress InitResultType = "progress"
)

type InitResult struct {
	Type    InitResultType  `json:"type"`
	Source  InitEventSource `json:"source,omitempty"`
	Message string         `json:"message,omitempty"`
	Label   string         `json:"label,omitempty"`
	Current int64          `json:"current,omitempty"`
	Total   int64          `json:"total,omitempty"`
	Percent int            `json:"percent,omitempty"`
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
			Source:  e.Source,
			Message: e.Message,
		}
	case InitProgressEvent:
		p := e.Progress
		return &InitResult{
			Type:    InitResultProgress,
			Source:  e.Source,
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

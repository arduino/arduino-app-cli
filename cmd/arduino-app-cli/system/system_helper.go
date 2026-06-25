// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

type jsonInitEvent struct {
	Type    string `json:"type"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message,omitempty"`
	Label   string `json:"label,omitempty"`
	Current int64  `json:"current,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Percent int    `json:"percent,omitempty"`
}

func newInitEventCallback(stdout io.Writer, jsonStream bool) orchestrator.InitEventCallback {
	var render orchestrator.InitEventCallback
	if jsonStream {
		enc := json.NewEncoder(stdout)
		render = func(e orchestrator.InitEvent) {
			switch e.Type {
			case orchestrator.InitLogEvent:
				_ = enc.Encode(jsonInitEvent{Type: "log", Source: string(e.Source), Message: e.Message})
			case orchestrator.InitProgressEvent:
				p := e.Progress
				var percentage int
				if p.Total > 0 {
					percentage = int(float64(p.Curr) / float64(p.Total) * 100)
				}
				_ = enc.Encode(jsonInitEvent{
					Type:    "progress",
					Source:  string(e.Source),
					Label:   p.Label,
					Current: p.Curr,
					Total:   p.Total,
					Percent: percentage,
				})
			}
		}
	} else {
		render = func(e orchestrator.InitEvent) {
			switch e.Type {
			case orchestrator.InitLogEvent:
				fmt.Fprintln(stdout, e.Message)
			case orchestrator.InitProgressEvent:
				p := e.Progress
				percentage := float64(p.Curr) / float64(p.Total) * 100
				fmt.Fprintf(stdout, "%s: %.0f%%\n", p.Label, percentage)
			}
		}
	}
	return throttleProgress(render)
}

func throttleProgress(next orchestrator.InitEventCallback) orchestrator.InitEventCallback {
	lastPct := map[string]int{}
	return func(e orchestrator.InitEvent) {
		if e.Type == orchestrator.InitProgressEvent {
			p := e.Progress
			if p.Total <= 0 {
				return
			}
			pct := int(float64(p.Curr) / float64(p.Total) * 100)
			if last, ok := lastPct[p.Label]; ok && last == pct {
				return
			}
			lastPct[p.Label] = pct
		}
		next(e)
	}
}

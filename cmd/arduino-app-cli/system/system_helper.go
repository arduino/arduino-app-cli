// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

var _ feedback.Result = (*orchestrator.InitResult)(nil)

func newInitEventCallback(printEvent func(feedback.Result)) orchestrator.InitEventCallback {
	return throttleProgress(func(e *orchestrator.InitResult) {
		printEvent(e)
	})
}

// throttleProgress drops the progress events that would render the same
// percentage twice in a row for a given label.
func throttleProgress(next func(*orchestrator.InitResult)) orchestrator.InitEventCallback {
	lastPct := map[string]int{}
	return func(event orchestrator.InitEvent) {
		e := orchestrator.NewInitResult(event)
		if e == nil {
			return
		}
		if e.Type == orchestrator.InitResultProgress {
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

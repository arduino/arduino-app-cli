// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"testing"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func TestThrottleProgress(t *testing.T) {
	progress := func(label string, curr, total int64) orchestrator.InitEvent {
		return orchestrator.InitEvent{
			Type:     orchestrator.InitProgressEvent,
			Progress: orchestrator.InitProgress{Label: label, Curr: curr, Total: total},
		}
	}
	logEvt := func(msg string) orchestrator.InitEvent {
		return orchestrator.InitEvent{Type: orchestrator.InitLogEvent, Message: msg}
	}

	var got []orchestrator.InitEvent
	cb := throttleProgress(func(e orchestrator.InitEvent) {
		got = append(got, e)
	})

	// Sequence: same integer percentage is collapsed; each new integer step and
	// every log line passes through.
	cb(progress("img", 0, 100))   // 0%  -> forwarded
	cb(progress("img", 4, 100))   // 4%  -> forwarded
	cb(progress("img", 49, 1000)) // 4%  -> dropped (same integer pct)
	cb(logEvt("hello"))           //     -> forwarded (logs always pass)
	cb(progress("img", 5, 100))   // 5%  -> forwarded
	cb(progress("img", 5, 0))     //     -> dropped (Total <= 0)
	cb(progress("other", 5, 100)) // 5%  -> forwarded (different label tracked separately)
	cb(logEvt("done"))            //     -> forwarded

	want := []orchestrator.InitEvent{
		progress("img", 0, 100),
		progress("img", 4, 100),
		logEvt("hello"),
		progress("img", 5, 100),
		progress("other", 5, 100),
		logEvt("done"),
	}

	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d:\n got:  %+v\n want: %+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

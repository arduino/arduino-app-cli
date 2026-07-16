// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
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

	var got []initResult
	cb := throttleProgress(func(e *initResult) {
		got = append(got, *e)
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

	want := []initResult{
		*fromInitEvent(progress("img", 0, 100)),
		*fromInitEvent(progress("img", 4, 100)),
		*fromInitEvent(logEvt("hello")),
		*fromInitEvent(progress("img", 5, 100)),
		*fromInitEvent(progress("other", 5, 100)),
		*fromInitEvent(logEvt("done")),
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

// The rendering of a result stream per output format is covered in the feedback
// package, here we only check that the events reach the stream, throttled.
func TestNewInitEventCallback(t *testing.T) {
	var got []feedback.Result
	cb := newInitEventCallback(func(res feedback.Result) {
		got = append(got, res)
	})

	cb(orchestrator.InitEvent{Type: orchestrator.InitLogEvent, Message: "pulling image"})
	cb(orchestrator.InitEvent{
		Type:     orchestrator.InitProgressEvent,
		Progress: orchestrator.InitProgress{Label: "img", Curr: 25, Total: 50},
	})
	// Same integer percentage as the previous event, dropped by the throttle.
	cb(orchestrator.InitEvent{
		Type:     orchestrator.InitProgressEvent,
		Progress: orchestrator.InitProgress{Label: "img", Curr: 250, Total: 500},
	})

	want := []string{"pulling image", "img: 50%"}
	if len(got) != len(want) {
		t.Fatalf("got %d results, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("result %d = %q, want %q", i, got[i].String(), want[i])
		}
	}
}

func TestInitEventJSON(t *testing.T) {
	tests := []struct {
		name  string
		event orchestrator.InitEvent
		want  string
	}{
		{
			name:  "log event",
			event: orchestrator.InitEvent{Type: orchestrator.InitLogEvent, Message: "pulling image"},
			want:  `{"type":"log","message":"pulling image"}`,
		},
		{
			name: "progress event",
			event: orchestrator.InitEvent{
				Type:     orchestrator.InitProgressEvent,
				Progress: orchestrator.InitProgress{Label: "img", Curr: 25, Total: 50},
			},
			want: `{"type":"progress","label":"img","current":25,"total":50,"percent":50}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each event must serialize to a single line, so that the JSON output
			// is a stream of JSON lines, one object per event.
			d, err := json.Marshal(fromInitEvent(tt.event).Data())
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if got := string(d); got != tt.want {
				t.Errorf("Data() = %s, want %s", got, tt.want)
			}
			if strings.Contains(string(d), "\n") {
				t.Errorf("Data() spans multiple lines: %s", d)
			}
		})
	}
}

func TestInitEventString(t *testing.T) {
	tests := []struct {
		name  string
		event orchestrator.InitEvent
		want  string
	}{
		{
			name:  "log event renders its message",
			event: orchestrator.InitEvent{Type: orchestrator.InitLogEvent, Message: "pulling image"},
			want:  "pulling image",
		},
		{
			name: "progress event renders label and percentage",
			event: orchestrator.InitEvent{
				Type:     orchestrator.InitProgressEvent,
				Progress: orchestrator.InitProgress{Label: "img", Curr: 25, Total: 50},
			},
			want: "img: 50%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fromInitEvent(tt.event).String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

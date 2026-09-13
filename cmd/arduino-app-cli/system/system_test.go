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

// The rendering of a result stream per output format is covered in the feedback
// package, here we only check that the events reach the stream.
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
			d, err := json.Marshal(orchestrator.NewInitResult(tt.event).Data())
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
			if got := orchestrator.NewInitResult(tt.event).String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package releasebuild

import (
	"sync"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// EventBroker fans out build progress events to every subscriber, so a single
// SSE stream can observe every build of every app. Each event carries the build
// id it belongs to, letting a subscriber filter by build.
type EventBroker struct {
	mu   sync.Mutex
	subs map[chan render.SSEEvent]struct{}
}

// The build events published to the build events stream. Each carries the optional
// build id it belongs to, so a subscriber can filter by build.
type (
	progressEvent struct {
		BuildID  string  `json:"build_id,omitempty"`
		Name     string  `json:"name"`
		Progress float32 `json:"progress"`
	}
	messageEvent struct {
		BuildID string `json:"build_id,omitempty"`
		Message string `json:"message"`
	}
	doneEvent struct {
		BuildID string `json:"build_id,omitempty"`
		Name    string `json:"name"`
		Target  string `json:"target"`
	}
	errorEvent struct {
		BuildID string            `json:"build_id,omitempty"`
		Code    render.SSEErrCode `json:"code"`
		Message string            `json:"message,omitempty"`
	}
)

// NewEventBroker returns a ready-to-use broker with no subscribers.
func NewEventBroker() *EventBroker {
	return &EventBroker{subs: make(map[chan render.SSEEvent]struct{})}
}

// Subscribe registers a subscriber and returns its event channel together with
// an unsubscribe function that must be called when done.
func (b *EventBroker) Subscribe() (<-chan render.SSEEvent, func()) {
	ch := make(chan render.SSEEvent, 128)

	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			delete(b.subs, ch)
			close(ch)
		})
	}
	return ch, unsubscribe
}

// PublishStreamMessage converts a build StreamMessage into a SSE event. It may hold
// a progress value, an info message, or both.
func (b *EventBroker) PublishStreamMessage(buildID string, item orchestrator.StreamMessage) {
	if p := item.GetProgress(); p != nil {
		b.publish(render.SSEEvent{Type: "progress", Data: progressEvent{BuildID: buildID, Name: p.Name, Progress: p.Progress}})
	}
	if item.GetData() != "" {
		b.publish(render.SSEEvent{Type: "message", Data: messageEvent{BuildID: buildID, Message: item.GetData()}})
	}
}

// PublishDone publishes the "done" event that a finished build emits.
func (b *EventBroker) PublishDone(buildID, name, target string) {
	b.publish(render.SSEEvent{Type: "done", Data: doneEvent{BuildID: buildID, Name: name, Target: target}})
}

// PublishError publishes the "error" event that a failed build emits.
func (b *EventBroker) PublishError(buildID string, code render.SSEErrCode, message string) {
	b.publish(render.SSEEvent{Type: "error", Data: errorEvent{BuildID: buildID, Code: code, Message: message}})
}

// publish delivers an event to every current subscriber. It never blocks the
// caller: an event is dropped for a subscriber whose buffer is full.
func (b *EventBroker) publish(event render.SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

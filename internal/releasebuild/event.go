// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package releasebuild

import (
	"sync"

	"github.com/arduino/arduino-app-cli/internal/render"
)

// EventBroker fans out build progress events to every subscriber, so a single
// SSE stream can observe every build of every app. Each event carries the build
// id it belongs to, letting a subscriber filter by build.
type EventBroker struct {
	mu   sync.Mutex
	subs map[chan render.SSEEvent]struct{}
}

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

// Publish delivers an event to every current subscriber. It never blocks the
// caller: an event is dropped for a subscriber whose buffer is full.
func (b *EventBroker) Publish(event render.SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

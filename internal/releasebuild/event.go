// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package releasebuild

import (
	"sync"

	"github.com/arduino/arduino-app-cli/internal/render"
)

// EventBroker fans out build progress events to per-app subscribers, so a
// single app-wide SSE stream can observe every build of that app. Each event
// carries the build id it belongs to, letting a subscriber filter by build.
type EventBroker struct {
	mu   sync.Mutex
	subs map[string]map[chan render.SSEEvent]struct{}
}

// NewEventBroker returns a ready-to-use broker with no subscribers.
func NewEventBroker() *EventBroker {
	return &EventBroker{subs: make(map[string]map[chan render.SSEEvent]struct{})}
}

// Subscribe registers a subscriber for the given app and returns its event
// channel together with an unsubscribe function that must be called when done.
func (b *EventBroker) Subscribe(appID string) (<-chan render.SSEEvent, func()) {
	ch := make(chan render.SSEEvent, 128)

	b.mu.Lock()
	if b.subs[appID] == nil {
		b.subs[appID] = make(map[chan render.SSEEvent]struct{})
	}
	b.subs[appID][ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if subs, ok := b.subs[appID]; ok {
				delete(subs, ch)
				close(ch)
				if len(subs) == 0 {
					delete(b.subs, appID)
				}
			}
		})
	}
	return ch, unsubscribe
}

// Publish delivers an event to every current subscriber of the app. It never
// blocks the caller: an event is dropped for a subscriber whose buffer is full.
func (b *EventBroker) Publish(appID string, event render.SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[appID] {
		select {
		case ch <- event:
		default:
		}
	}
}

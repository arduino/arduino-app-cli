package handlers

import (
	"sync"

	"github.com/arduino/arduino-app-cli/pkg/render"
)

// UpdateEventsBroker manages subscribers and publishes SSE events to all of them.
type UpdateEventsBroker struct {
	mu   sync.Mutex
	subs map[chan render.SSEEvent]struct{}
}

func NewUpdateEventsBroker() *UpdateEventsBroker {
	return &UpdateEventsBroker{
		subs: make(map[chan render.SSEEvent]struct{}),
	}
}

// Subscribe returns the SSE event channel for a subscriber.
func (b *UpdateEventsBroker) Subscribe() chan render.SSEEvent {
	eventCh := make(chan render.SSEEvent, 100)
	b.mu.Lock()
	b.subs[eventCh] = struct{}{}
	b.mu.Unlock()
	return eventCh
}

// Unsubscribe removes the SSE event channel for a subscriber.
func (b *UpdateEventsBroker) Unsubscribe(eventCh chan render.SSEEvent) {
	b.mu.Lock()
	delete(b.subs, eventCh)
	close(eventCh)
	b.mu.Unlock()
}

func (b *UpdateEventsBroker) PublishLog(line string) {
	b.publish(render.SSEEvent{Type: "log", Data: line})
}

// PublishError sends an SSE error event with the given SSEErrorData to all subscribers.
func (b *UpdateEventsBroker) PublishError(err render.SSEErrorData) {
	b.publish(render.SSEEvent{Type: "error", Data: err})
}

// Restarting publishes a "restarting" SSE event to all subscribers.
func (b *UpdateEventsBroker) Restarting() {
	b.publish(render.SSEEvent{Type: "restarting", Data: "Upgrade completed. Restarting ..."})
}

// Publish sends the SSE event to all event subscribers.
func (b *UpdateEventsBroker) publish(event render.SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

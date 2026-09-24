// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package releasebuild

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/render"
)

// event builds a distinguishable event: the payload carries the sequence number,
// so a test can tell events apart and assert their order.
func event(n int) render.SSEEvent {
	return render.SSEEvent{Type: "progress", Data: n}
}

// receive returns the next event, or fails if none arrives before the timeout, so
// a stuck delivery fails the test instead of hanging it.
func receive(t *testing.T, ch <-chan render.SSEEvent) render.SSEEvent {
	t.Helper()
	select {
	case e, ok := <-ch:
		require.True(t, ok, "the channel was closed before an event arrived")
		return e
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for an event")
		return render.SSEEvent{}
	}
}

// expectNoEvent fails if an event arrives on an open channel within a short window.
func expectNoEvent(t *testing.T, ch <-chan render.SSEEvent) {
	t.Helper()
	select {
	case e, ok := <-ch:
		if ok {
			t.Fatalf("received an unexpected event: %+v", e)
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func TestEventBrokerDeliversToSubscriber(t *testing.T) {
	broker := NewEventBroker()
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	want := event(1)
	broker.publish(want)

	require.Equal(t, want, receive(t, ch))
}

func TestEventBrokerFansOutToEverySubscriber(t *testing.T) {
	broker := NewEventBroker()
	chA, unsubscribeA := broker.Subscribe()
	defer unsubscribeA()
	chB, unsubscribeB := broker.Subscribe()
	defer unsubscribeB()

	want := event(1)
	broker.publish(want)

	require.Equal(t, want, receive(t, chA))
	require.Equal(t, want, receive(t, chB))
}

func TestEventBrokerPublishWithoutSubscribersIsNoOp(t *testing.T) {
	broker := NewEventBroker()
	require.NotPanics(t, func() { broker.publish(event(1)) })
}

func TestEventBrokerPreservesOrder(t *testing.T) {
	broker := NewEventBroker()
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	want := []render.SSEEvent{event(1), event(2), event(3)}
	for _, e := range want {
		broker.publish(e)
	}

	for _, e := range want {
		require.Equal(t, e, receive(t, ch))
	}
}

func TestEventBrokerLateSubscriberMissesEarlierEvents(t *testing.T) {
	broker := NewEventBroker()

	// Published before anyone subscribes: the broker keeps no history.
	broker.publish(event(1))

	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()
	broker.publish(event(2))

	require.Equal(t, event(2), receive(t, ch))
	expectNoEvent(t, ch)
}

func TestEventBrokerUnsubscribeStopsDeliveryAndClosesChannel(t *testing.T) {
	broker := NewEventBroker()
	ch, unsubscribe := broker.Subscribe()

	unsubscribe()

	_, ok := <-ch
	require.False(t, ok, "the channel must be closed after unsubscribe")

	// Publishing with no subscribers left is a safe no-op.
	require.NotPanics(t, func() { broker.publish(event(1)) })
}

func TestEventBrokerUnsubscribeIsIdempotent(t *testing.T) {
	broker := NewEventBroker()
	_, unsubscribe := broker.Subscribe()

	unsubscribe()
	require.NotPanics(t, unsubscribe, "a second unsubscribe must not panic")
}

func TestEventBrokerUnsubscribeOneKeepsTheOther(t *testing.T) {
	broker := NewEventBroker()
	chA, unsubscribeA := broker.Subscribe()
	chB, unsubscribeB := broker.Subscribe()
	defer unsubscribeB()

	unsubscribeA()

	want := event(1)
	broker.publish(want)

	require.Equal(t, want, receive(t, chB))
	_, ok := <-chA
	require.False(t, ok, "the unsubscribed channel must be closed")
}

func TestEventBrokerPublishNeverBlocksWhenBufferFull(t *testing.T) {
	broker := NewEventBroker()
	// The subscriber never reads, so its buffer fills and further events are dropped.
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	const published = 1000
	done := make(chan struct{})
	go func() {
		for i := 0; i < published; i++ {
			broker.publish(event(i))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked while the subscriber buffer was full")
	}

	received := 0
	for {
		select {
		case <-ch:
			received++
			continue
		default:
		}
		break
	}
	require.Greater(t, received, 0, "the subscriber should have received the buffered events")
	require.Less(t, received, published, "events beyond the buffer must be dropped, not queued")
}

func TestEventBrokerPublishDone(t *testing.T) {
	broker := NewEventBroker()
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	broker.PublishDone("build-1", "my-app", "unoq")

	got := receive(t, ch)
	require.Equal(t, "done", got.Type)
	require.Equal(t, doneEvent{BuildID: "build-1", Name: "my-app", Target: "unoq"}, got.Data)
}

func TestEventBrokerPublishError(t *testing.T) {
	broker := NewEventBroker()
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	broker.PublishError("build-1", render.InternalServiceErr, "boom")

	got := receive(t, ch)
	require.Equal(t, "error", got.Type)
	require.Equal(t, errorEvent{BuildID: "build-1", Code: render.InternalServiceErr, Message: "boom"}, got.Data)
}

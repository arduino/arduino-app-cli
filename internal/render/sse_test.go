// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSendDoesNotBlockAfterClientDisconnect covers the deadlock that made a
// failed model download wedge the daemon until it was restarted: once the
// client goes away the stream stops draining messageCh, and a Send from the
// docker output goroutine blocked on it forever, so the handler never returned
// and its deferred Close was never reached.
func TestSendDoesNotBlockAfterClientDisconnect(t *testing.T) {
	for _, tc := range []struct {
		name string
		send func(s *SSEStream)
	}{
		{"Send", func(s *SSEStream) { s.Send(SSEEvent{Type: "progress", Data: "1%"}) }},
		{"SendError", func(s *SSEStream) { s.SendError(SSEErrorData{Code: InternalServiceErr, Message: "boom"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			w := &syncResponseWriter{header: http.Header{}}

			stream, err := NewSSEStream(ctx, w)
			require.NoError(t, err)

			// The client disconnects: this is what cancels the request context.
			cancel()

			// loop() writes the farewell events on its way out, so their arrival
			// means it has stopped reading messageCh.
			require.Eventually(t, func() bool {
				return strings.Contains(w.String(), "SERVER_CLOSED")
			}, 2*time.Second, time.Millisecond, "stream never stopped")

			done := make(chan struct{})
			go func() {
				defer close(done)
				tc.send(stream)
			}()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("send blocked after the client disconnected")
			}
		})
	}
}

// syncResponseWriter is a minimal http.ResponseWriter satisfying the sseFlusher
// interface, safe to read while the stream goroutine writes to it.
type syncResponseWriter struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	header http.Header
}

func (w *syncResponseWriter) Header() http.Header { return w.header }

func (w *syncResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *syncResponseWriter) WriteHeader(int) {}

func (w *syncResponseWriter) Flush() {}

func (w *syncResponseWriter) SetWriteDeadline(time.Time) error { return nil }

func (w *syncResponseWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

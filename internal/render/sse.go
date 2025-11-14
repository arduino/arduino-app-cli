// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package render

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

type SSEErrCode string

const (
	InternalServiceErr SSEErrCode = "INTERNAL_SERVER_ERROR"
)

type SSEErrorData struct {
	Code    SSEErrCode `json:"code"`
	Message string     `json:"message,omitempty"`
}

type SSEEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func NewErrorEvent(data any) SSEEvent {
	return SSEEvent{
		Type: "error",
		Data: data,
	}
}

type sseFlusher interface {
	http.Flusher
	io.Writer
}

type SSEStream struct {
	sseFlusher sseFlusher

	heartbeatInterval time.Duration
	messageCh         chan SSEEvent
	isClosing         atomic.Bool

	shutdownFn context.CancelFunc
	stoppedCh  chan struct{}
	ctx        context.Context
}

func NewSSEStream(ctx context.Context, w http.ResponseWriter) (*SSEStream, error) {
	// set the write deadline to avoid the connection to be closed by the server
	err := w.(interface{ SetWriteDeadline(time.Time) error }).SetWriteDeadline(time.Time{})
	if err != nil {
		return nil, fmt.Errorf("failed to set write deadline: %w", err)
	}

	flusher, ok := w.(sseFlusher)
	if !ok {
		return nil, fmt.Errorf("failed to cast http.ResponseWriter to http.Flusher")
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")

	const maxStreamTime = 24 * time.Hour

	ctx, cancel := context.WithTimeout(ctx, maxStreamTime)
	s := &SSEStream{
		sseFlusher:        flusher,
		heartbeatInterval: 30 * time.Second,
		messageCh:         make(chan SSEEvent),
		isClosing:         atomic.Bool{},
		shutdownFn:        cancel,
		stoppedCh:         make(chan struct{}),
		ctx:               ctx,
	}

	go s.loop()

	return s, nil
}

func (s *SSEStream) loop() {
	defer func() {
		// This is kept for backward compatibility. We should remove this in the future.
		_ = s.send(SSEEvent{Type: "error", Data: SSEErrorData{Code: "SERVER_CLOSED"}})

		// Notify the client that the stream is closed
		_ = s.send(SSEEvent{Type: "close", Data: "Stream closed by server"})
		close(s.stoppedCh)
	}()

	for {
		select {
		case <-s.ctx.Done():
			slog.Debug("stream SSE request context done")
			return
		case <-time.After(s.heartbeatInterval):
			if err := s.heartbeat(); err != nil {
				slog.Error("failed to send ping", slog.Any("error", err))
				return
			}
		case event, canProduce := <-s.messageCh:
			if !canProduce {
				slog.Debug("events channel is closed")
				return
			}
			if err := s.send(event); err != nil {
				slog.Debug("failed to send SSE event", slog.String("event", event.Type), slog.Any("error", err))
				return
			}
		}
	}
}

func (s *SSEStream) Send(event SSEEvent) {
	if s.isClosing.Load() {
		slog.Debug("SSE stream is closing, ignoring event", slog.String("event", event.Type))
		return
	}
	s.messageCh <- event
}

func (s *SSEStream) SendError(event SSEErrorData) {
	if s.isClosing.Load() {
		slog.Debug("SSE stream is closing, ignoring event", slog.String("event", "error"))
		return
	}
	s.messageCh <- SSEEvent{Type: "error", Data: event}
}

func (s *SSEStream) Close() {
	if !s.isClosing.CompareAndSwap(false, true) {
		// already closed
		return
	}
	s.shutdownFn()
	<-s.stoppedCh
}

func (s *SSEStream) send(e SSEEvent) error {
	if e.Type != "" {
		if _, err := s.sseFlusher.Write([]byte("event: " + e.Type + "\n")); err != nil {
			return err
		}
	}
	if _, err := s.sseFlusher.Write([]byte("data: ")); err != nil {
		return err
	}
	if err := json.NewEncoder(s.sseFlusher).Encode(e.Data); err != nil {
		return err
	}

	// Json add a default \n at the end of the json, so we need to add another one
	if _, err := s.sseFlusher.Write([]byte("\n")); err != nil {
		return err
	}

	s.sseFlusher.Flush()
	return nil
}

func (s *SSEStream) heartbeat() error {
	if s.isClosing.Load() {
		slog.Debug("SSE stream is closing, ignoring event", slog.String("event", "error"))
		return nil
	}
	if _, err := s.sseFlusher.Write([]byte("event: heartbeat\n\n")); err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	s.sseFlusher.Flush()
	return nil
}

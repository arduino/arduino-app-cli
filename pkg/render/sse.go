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
	ServerCloseErr     SSEErrCode = "SERVER_CLOSED"
)

type SSEErrorData struct {
	Code    SSEErrCode `json:"code"`
	Message string     `json:"message,omitempty"`
}

type SSEEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
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
		_ = s.send(SSEEvent{Type: "error", Data: SSEErrorData{Code: ServerCloseErr}})
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

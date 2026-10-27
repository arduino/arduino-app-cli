// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"syscall/js"

	"github.com/arduino/arduino-app-cli/internal/editor"
)

// serveEditor runs an editor session, the daemon's /v1/edit, on a MessagePort the page
// handed over, since a service worker cannot accept a WebSocket. It returns at once, as
// a js.Func must.
func serveEditor(port js.Value) {
	stream := newPortStream(port)
	go func() {
		if err := editor.Serve(context.Background(), editor.Config{Logger: slog.Default()}, stream); err != nil {
			slog.Error("editor session", slog.String("err", err.Error()))
		}
	}()
}

// portStream carries a session's JSON-RPC messages over a MessagePort, a JSON text per
// message as a WebSocket carries one per frame. The page hangs up by posting null, or by
// closing its end where the browser reports it (the port's close event).
type portStream struct {
	port    js.Value
	onEvent js.Func
	wake    chan struct{}

	mu     sync.Mutex
	queue  []string
	closed bool
}

func newPortStream(port js.Value) *portStream {
	s := &portStream{port: port, wake: make(chan struct{}, 1)}
	// It only queues: a js.Func must not block.
	s.onEvent = js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		data := event.Get("data")
		s.mu.Lock()
		switch {
		case event.Get("type").String() == "message" && data.Type() == js.TypeString:
			s.queue = append(s.queue, data.String())
		case event.Get("type").String() == "close" || data.Type() == js.TypeNull:
			s.closed = true
		}
		s.mu.Unlock()
		s.signal()
		return nil
	})
	port.Call("addEventListener", "message", s.onEvent)
	port.Call("addEventListener", "close", s.onEvent)
	port.Call("start")
	return s
}

func (s *portStream) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *portStream) ReadObject(v any) error {
	for {
		s.mu.Lock()
		if len(s.queue) > 0 {
			data := s.queue[0]
			s.queue = s.queue[1:]
			s.mu.Unlock()
			return json.Unmarshal([]byte(data), v)
		}
		closed := s.closed
		s.mu.Unlock()
		if closed {
			return io.EOF
		}
		<-s.wake
	}
}

func (s *portStream) WriteObject(obj any) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	s.mu.Lock()
	port, closed := s.port, s.closed
	s.mu.Unlock()
	if closed {
		return io.ErrClosedPipe
	}
	port.Call("postMessage", string(b))
	return nil
}

func (s *portStream) Close() error {
	s.mu.Lock()
	if s.port.IsUndefined() {
		s.mu.Unlock()
		return nil
	}
	port := s.port
	s.port = js.Undefined()
	s.closed = true
	s.mu.Unlock()

	port.Call("removeEventListener", "message", s.onEvent)
	port.Call("removeEventListener", "close", s.onEvent)
	port.Call("close")
	s.onEvent.Release()
	s.signal()
	return nil
}

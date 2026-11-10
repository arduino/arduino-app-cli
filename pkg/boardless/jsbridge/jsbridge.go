// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

// Package jsbridge serves an http.Handler to a page, with no port: the page hands each
// request to Serve and gets the response back through a sink, chunk by chunk.
package jsbridge

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"syscall/js"
	"time"
)

// Serve runs request through handler on its own goroutine and returns the function that
// cancels it.
//
// request is {method, url, headers: [[name, value]…], body: Uint8Array | null}; sink is
// {head(status, headers), chunk(Uint8Array), end(message?)}.
func Serve(handler http.Handler, request, sink js.Value) js.Func {
	ctx, cancel := context.WithCancel(context.Background())
	cancelFunc := js.FuncOf(func(js.Value, []js.Value) any {
		cancel()
		return nil
	})

	var body []byte
	if b := request.Get("body"); b.Truthy() {
		body = make([]byte, b.Get("byteLength").Int())
		js.CopyBytesToGo(body, b)
	}
	req, err := http.NewRequestWithContext(ctx, request.Get("method").String(), "http://localhost"+request.Get("url").String(), bytes.NewReader(body))
	if err != nil {
		cancel()
		sink.Call("end", err.Error())
		return cancelFunc
	}
	req.RemoteAddr = "127.0.0.1:0"
	req.RequestURI = req.URL.RequestURI()
	if headers := request.Get("headers"); headers.Truthy() {
		for i := 0; i < headers.Length(); i++ {
			pair := headers.Index(i)
			req.Header.Add(pair.Index(0).String(), pair.Index(1).String())
		}
	}

	go func() {
		defer cancel()
		w := &sinkWriter{sink: sink, header: http.Header{}}
		handler.ServeHTTP(w, req)
		w.WriteHeader(http.StatusOK)
		_ = w.call("end")
	}()
	return cancelFunc
}

// sinkWriter hands the response to the page as it is written, with nothing buffered, as
// an event stream needs.
type sinkWriter struct {
	sink        js.Value
	header      http.Header
	wroteHeader bool
	gone        error
}

// call calls a sink method. What the page throws, such as for a cancelled stream, comes
// back as an error to the handler, not as a panic; no chunk follows it.
func (w *sinkWriter) call(method string, args ...any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			thrown, ok := r.(js.Error)
			if !ok {
				panic(r)
			}
			w.gone = fmt.Errorf("the page took no more of the response: %w", thrown)
			err = w.gone
		}
	}()
	w.sink.Call(method, args...)
	return nil
}

func (w *sinkWriter) Header() http.Header { return w.header }

func (w *sinkWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	pairs := []any{}
	for name, values := range w.header {
		for _, value := range values {
			pairs = append(pairs, []any{name, value})
		}
	}
	_ = w.call("head", status, js.ValueOf(pairs))
}

func (w *sinkWriter) Write(b []byte) (int, error) {
	if w.gone != nil {
		return 0, w.gone
	}
	w.WriteHeader(http.StatusOK)
	chunk := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(chunk, b)
	if err := w.call("chunk", chunk); err != nil {
		return 0, err
	}
	return len(b), nil
}

// Flush sends the head: an event stream flushes before its first event.
func (w *sinkWriter) Flush() { w.WriteHeader(http.StatusOK) }

// A page has no connection to time out.
func (w *sinkWriter) SetWriteDeadline(time.Time) error { return nil }
func (w *sinkWriter) SetReadDeadline(time.Time) error  { return nil }

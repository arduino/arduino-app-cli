// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

// Command wasmdaemon is the daemon built for js/wasm, for a page with no board: the same
// router and handlers as `arduino-app-cli daemon`, on the files the page gives Go's os
// package, with no docker and no arduino-cli. What needs them answers that it needs a
// board (dockerhelper.ErrNoDocker, orchestrator.ErrNoArduinoCli).
//
// It listens on no port. The page calls, for each request,
//
//	globalThis.arduinoAppCli.serve(request, sink) → cancel()
//
// with request {method, url, headers: [[name, value]…], body: Uint8Array | null} and
// sink {head(status, headers), chunk(Uint8Array), end(message?)}: streams such as the
// app events arrive chunk by chunk, and cancel() ends a request the page no longer reads.
// The page learns that the daemon is up from globalThis.arduinoAppCliReady(), if defined.
//
// The editor service, a WebSocket at the daemon's /v1/edit, takes a MessagePort here:
//
//	globalThis.arduinoAppCli.editor(port)
//
// runs one session on it, a JSON-RPC message per port message (editor.Serve).
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"syscall/js"
	"time"

	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/internal/httprecover"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/releasebuild"
	"github.com/arduino/arduino-app-cli/internal/update"
)

// Version is set at build time, as the daemon's is.
var Version = "0.0.0-dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg := f.Must(config.NewFromEnv())
	servicelocator.Init(cfg)

	// As the daemon's httpHandler wires it, with an updater that finds nothing: a page
	// updates with its site, not with apt.
	router := api.NewHTTPRouter(
		servicelocator.GetDockerClient(),
		Version,
		update.NewManager(noUpdates{}, noUpdates{}),
		servicelocator.GetProvisioner(),
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
		servicelocator.GetServicesIndex(),
		servicelocator.GetBrickService(),
		servicelocator.GetAppIDProvider(),
		servicelocator.GetPlatform(),
		cfg,
		releasebuild.NewEventBroker(),
		nil,
	)
	handler := httprecover.RecoverPanic(router)

	js.Global().Set("arduinoAppCli", js.ValueOf(map[string]any{
		"version": Version,
		"serve":   js.FuncOf(func(_ js.Value, args []js.Value) any { return serve(handler, args[0], args[1]) }),
		"editor":  js.FuncOf(func(_ js.Value, args []js.Value) any { serveEditor(args[0]); return nil }),
	}))
	if ready := js.Global().Get("arduinoAppCliReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

// serve runs one request through the handler, on its own goroutine, and returns the
// function that cancels it.
func serve(handler http.Handler, request, sink js.Value) js.Func {
	ctx, cancel := context.WithCancel(context.Background())
	var cancelFunc js.Func
	cancelFunc = js.FuncOf(func(js.Value, []js.Value) any {
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

// sinkWriter hands the response to the page as it is written: the head once, then each
// write as a chunk, with nothing buffered, which is what an event stream asks for.
type sinkWriter struct {
	sink        js.Value
	header      http.Header
	wroteHeader bool
	gone        error
}

// call calls one of the sink's methods. What the page throws, such as a chunk for a stream
// it has already cancelled, comes back as an error for the handler, not as a panic that
// ends the daemon; no chunk goes to the sink after it.
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

// Flush sends the head, as a server's flush commits it: an event stream flushes before its first event.
func (w *sinkWriter) Flush() { w.WriteHeader(http.StatusOK) }

// The deadlines a server stream sets on its connection: a page has no connection to time out.
func (w *sinkWriter) SetWriteDeadline(time.Time) error { return nil }
func (w *sinkWriter) SetReadDeadline(time.Time) error  { return nil }

// noUpdates is the apt and the arduino-cli of a page: nothing to upgrade.
type noUpdates struct{}

var errNoUpdates = errors.New("a page updates with its site: there is nothing to upgrade here")

func (noUpdates) ListUpgradablePackages(context.Context, func(update.UpgradablePackage) bool) ([]update.UpgradablePackage, error) {
	return nil, nil
}

func (noUpdates) UpgradePackages(context.Context, []update.PackageInfo, update.EventCallback) error {
	return errNoUpdates
}

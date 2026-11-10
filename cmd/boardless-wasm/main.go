// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

// Command boardless-wasm is the Boardless API for the PWA, as WebAssembly. It listens on
// no port. The page mounts its fs (ZenFS over OPFS) before it runs this, then calls
//
//	arduinoAppCli.start({board, appsDir, dataDir, customModelsDir}) → Promise, rejected with why it cannot serve
//	arduinoAppCli.serve(request, sink) → cancel()
//
// start can be called again to switch board. The page learns that the module is up from
// globalThis.arduinoAppCliReady(), if defined.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"syscall/js"

	"github.com/arduino/arduino-app-cli/internal/httprecover"
	"github.com/arduino/arduino-app-cli/pkg/boardless"
	"github.com/arduino/arduino-app-cli/pkg/boardless/jsbridge"
)

// Version is set at build time.
var Version = "0.0.0-dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	var handler atomic.Pointer[http.Handler]

	// start returns a Promise: NewHandler reads the fs, and Go's fs on js waits for the
	// page's event loop, which a synchronous callback would block.
	start := js.FuncOf(func(_ js.Value, args []js.Value) any {
		opts := boardless.Options{Version: Version}
		if len(args) > 0 && args[0].Truthy() {
			str := func(key string) string {
				if v := args[0].Get(key); v.Truthy() {
					return v.String()
				}
				return ""
			}
			opts.Board = str("board")
			opts.AppsDir = str("appsDir")
			opts.DataDir = str("dataDir")
			opts.CustomModelsDir = str("customModelsDir")
		}
		return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, p []js.Value) any {
			resolve, reject := p[0], p[1]
			go func() {
				h, err := boardless.NewHandler(opts)
				if err != nil {
					reject.Invoke(js.Global().Get("Error").New(err.Error()))
					return
				}
				h = httprecover.RecoverPanic(h)
				handler.Store(&h)
				resolve.Invoke()
			}()
			return nil
		}))
	})
	serve := js.FuncOf(func(_ js.Value, args []js.Value) any {
		h := handler.Load()
		if h == nil {
			args[1].Call("end", "not started: call arduinoAppCli.start first")
			return js.FuncOf(func(js.Value, []js.Value) any { return nil })
		}
		return jsbridge.Serve(*h, args[0], args[1])
	})

	js.Global().Set("arduinoAppCli", js.ValueOf(map[string]any{
		"version": Version,
		"start":   start,
		"serve":   serve,
	}))
	if ready := js.Global().Get("arduinoAppCliReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package monitor

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"time"

	"go.bug.st/f"
)

const defaultArduinoRouterMonitorAddress = "127.0.0.1:7500"

func NewMonitorHandler(rw io.ReadWriteCloser, address ...string) (func(), error) {
	f.Assert(len(address) <= 1, "NewMonitorHandler accepts at most one address argument")

	addr := defaultArduinoRouterMonitorAddress
	if len(address) == 1 {
		addr = address[0]
	}

	// Connect to monitor
	monitor, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return nil, err
	}

	return func() {
		monitorStream(monitor, rw)
	}, nil
}

func monitorStream(mon net.Conn, rw io.ReadWriteCloser) {
	logSocketError := func(msg string, err error) {
		if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
			slog.Error(msg, slog.String("error", err.Error()))
		}
	}
	go func() {
		defer mon.Close()
		defer rw.Close()
		buff := [1024]byte{}
		for {
			// Read from reader and write to monitor
			n, err := rw.Read(buff[:])
			if err != nil {
				logSocketError("Error reading from websocket", err)
				return
			}
			if _, err := mon.Write(buff[:n]); err != nil {
				logSocketError("Error writing to monitor", err)
				return
			}
		}
	}()
	go func() {
		defer mon.Close()
		defer rw.Close()
		buff := [1024]byte{}
		for {
			// Read from monitor and write to writer
			n, err := mon.Read(buff[:])
			if err != nil {
				logSocketError("Error reading from monitor", err)
				return
			}

			if _, err := rw.Write(buff[:n]); err != nil {
				logSocketError("Error writing to buffer", err)
				return
			}
		}
	}()
}

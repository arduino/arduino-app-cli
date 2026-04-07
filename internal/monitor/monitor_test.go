// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package monitor

import (
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arduino/arduino-app-cli/pkg/x/ports"
)

func TestMonitorHandler(t *testing.T) {
	addr := startEcoMonitor(t)

	rIn, wIn, rwOut := getReadWriteCloser()

	handler, err := NewMonitorHandler(rwOut, addr.String())
	assert.NoError(t, err)
	go handler()

	// Write data to the pipe writer
	message := "Hello, Monitor!"
	n, err := wIn.Write([]byte(message))
	assert.NoError(t, err)
	assert.Equal(t, len(message), n)

	// Read data from the pipe reader
	buf := [128]byte{}
	n, err = rIn.Read(buf[:])
	assert.NoError(t, err)
	assert.Equal(t, len(message), n)
	assert.Equal(t, message, string(buf[:n]))
}

func getReadWriteCloser() (io.Reader, io.Writer, io.ReadWriteCloser) {
	rOut, wIn := io.Pipe()
	rIn, wOut := io.Pipe()

	type pipeReadWriteCloser struct {
		io.Reader
		io.Writer
		io.Closer
	}
	pr := &pipeReadWriteCloser{
		Reader: rOut,
		Writer: wOut,
		Closer: io.NopCloser(nil),
	}
	return rIn, wIn, pr
}

func startEcoMonitor(t *testing.T) net.Addr {
	t.Helper()

	port, err := ports.GetAvailable()
	assert.NoError(t, err)

	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	assert.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn) // Echo server
			}()
		}
	}()

	return ln.Addr()
}

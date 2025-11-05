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

package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func monitorStream(mon net.Conn, ws *websocket.Conn) {
	logWebsocketError := func(msg string, err error) {
		// Do not log simple close or interruption errors
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
			if e, ok := err.(*websocket.CloseError); ok {
				slog.Error(msg, slog.String("closecause", fmt.Sprintf("%d: %s", e.Code, err)))
			} else {
				slog.Error(msg, slog.String("error", err.Error()))
			}
		}
	}
	logSocketError := func(msg string, err error) {
		if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
			slog.Error(msg, slog.String("error", err.Error()))
		}
	}
	go func() {
		defer mon.Close()
		defer ws.Close()
		for {
			// Read from websocket and write to monitor
			_, msg, err := ws.ReadMessage()
			if err != nil {
				logWebsocketError("Error reading from websocket", err)
				return
			}
			if _, err := mon.Write(msg); err != nil {
				logSocketError("Error writing to monitor", err)
				return
			}
		}
	}()
	go func() {
		defer mon.Close()
		defer ws.Close()
		buff := [1024]byte{}
		for {
			// Read from monitor and write to websocket
			n, err := mon.Read(buff[:])
			if err != nil {
				logSocketError("Error reading from monitor", err)
				return
			}

			if err := ws.WriteMessage(websocket.BinaryMessage, buff[:n]); err != nil {
				logWebsocketError("Error writing to websocket", err)
				return
			}
		}
	}()
}

func splitOrigin(origin string) (scheme, host, port string, err error) {
	parts := strings.SplitN(origin, "://", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid origin format: %s", origin)
	}
	scheme = parts[0]
	hostPort := parts[1]
	hostParts := strings.SplitN(hostPort, ":", 2)
	host = hostParts[0]
	if len(hostParts) == 2 {
		port = hostParts[1]
	} else {
		port = "*"
	}
	return scheme, host, port, nil
}

func checkOrigin(origin string, allowedOrigins []string) bool {
	scheme, host, port, err := splitOrigin(origin)
	if err != nil {
		slog.Error("WebSocket origin check failed", slog.String("origin", origin), slog.String("error", err.Error()))
		return false
	}
	for _, allowed := range allowedOrigins {
		allowedScheme, allowedHost, allowedPort, err := splitOrigin(allowed)
		if err != nil {
			panic(err)
		}
		if allowedScheme != scheme {
			continue
		}
		if allowedHost != host && allowedHost != "*" {
			continue
		}
		if allowedPort != port && allowedPort != "*" {
			continue
		}
		return true
	}
	slog.Error("WebSocket origin check failed", slog.String("origin", origin))
	return false
}

func HandleMonitorWS(allowedOrigins []string) http.HandlerFunc {
	// Do a dry-run of checkorigin, so it can panic if misconfigured now, not on first request
	_ = checkOrigin("http://localhost", allowedOrigins)

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return checkOrigin(r.Header.Get("Origin"), allowedOrigins)
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Connect to monitor
		mon, err := net.DialTimeout("tcp", "127.0.0.1:7500", time.Second)
		if err != nil {
			slog.Error("Unable to connect to monitor", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusServiceUnavailable, models.ErrorResponse{Details: "Unable to connect to monitor: " + err.Error()})
			return
		}

		// Upgrade the connection to websocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Remember to close monitor connection if websocket upgrade fails.
			mon.Close()

			slog.Error("Failed to upgrade connection", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to upgrade connection: " + err.Error()})
			return
		}

		// Now the connection is managed by the websocket library, let's move the handlers in the goroutine
		go monitorStream(mon, conn)

		// and return nothing to the http library
	}
}
